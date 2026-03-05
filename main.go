/*
 *
 * title:   gotorrent
 * author:  Andrew Souza
 * license: GPLv3
 *
 */
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand/v2"
	"sync"
	"time"

	"net/url"
	"os"

	"github.com/drewslam/gotorrent/pkg/peer_protocol"
	"github.com/drewslam/gotorrent/pkg/storage"
	"github.com/drewslam/gotorrent/pkg/torrent"
	"github.com/drewslam/gotorrent/pkg/tracker"
)

const MaxPeerConnections uint32 = 40

var (
	ActivePeerConnections = make(map[uint32]*peer_protocol.PeerConn, MaxPeerConnections)
	RestrictedUntil       = make(map[string]time.Time)
	connectionMutex       sync.RWMutex
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <torrent-file>\n", os.Args[0])
		os.Exit(1)
	}

	tor, err := LoadTorrent(os.Args)
	if err != nil {
		log.Fatalf("loadTorrent failure: %v", err)
	}

	peer := tracker.NewPeer()
	req := tracker.NewRequest(tor.InfoHash(), peer, tor.FileSize())

	rs, err := AnnounceToTrackers(tor.Announce, req)
	if err != nil {
		log.Fatalf("announceToTrackers failure: %v", err)
	}

	PrintTorResponseInfo(tor, rs)

	storage, err := storage.NewFileStorage(tor, tor.Info.Name)
	if err != nil {
		log.Fatalf("NewFileStorage failure: %v", err)
	}

	pieceMgr := peer_protocol.NewPieceManager(tor, storage)

	if err := storage.Allocate(); err != nil {
		log.Fatalf("allocation failure: %v", err)
	}

	go ManagePeerConnections(rs, req, pieceMgr)

	select {}
}

func LoadTorrent(input []string) (*torrent.Torrent, error) {
	rawBytes, err := os.ReadFile(input[1])
	if err != nil {
		return nil, fmt.Errorf("failed to open torrent file: %v\n", err)
	}

	return torrent.DecodeTorrentFile(rawBytes)
}

func AnnounceToTrackers(announceList []string, req *tracker.Request) (*tracker.Response, error) {
	for i, announce := range announceList {
		ur, err := url.Parse(announce)
		if err != nil {
			if i == 0 {
				fmt.Printf("invalid primary tracker URL: %v\n", err)
			}
			continue
		}

		rs, err := req.Announce(ur)
		if err != nil {
			if i == 0 {
				fmt.Printf("primary tracker announce failure: %v\n", err)
			}
			continue
		}

		if len(rs.Peers) > 0 {
			return rs, nil
		}
	}

	return nil, fmt.Errorf("no peers found from any tracker\n")
}

func ManagePeerConnections(rs *tracker.Response, req *tracker.Request, pieceMgr *peer_protocol.PieceManager) {
	ConnectToPeers(rs, req, pieceMgr)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		connectionMutex.RLock()
		activeCount := len(ActivePeerConnections)
		connectionMutex.RUnlock()

		if activeCount < int(MaxPeerConnections) {
			fmt.Printf("replenishing connections: %d active\n", activeCount)
			ConnectToPeers(rs, req, pieceMgr)
		}
 	}
}

func ConnectToPeers(rs *tracker.Response, req *tracker.Request, pieceMgr *peer_protocol.PieceManager) {
	peers := rs.Peers
	numToConnect := min(int(MaxPeerConnections), len(peers))

	for {
		connectionMutex.RLock()
		activeCount := len(ActivePeerConnections)
		connectionMutex.RUnlock()

		if activeCount >= numToConnect {
			break
		}

		randomIndex := findUnusedPeerIndex(peers)
		if randomIndex == -1 {
			fmt.Printf("failed to find unique peer, stopping at %d connections\n", len(ActivePeerConnections))
			break
		}

		addr := peers[randomIndex].Address()

		connectionMutex.RLock()
		expiry, banned := RestrictedUntil[addr]
		connectionMutex.RUnlock()

		if banned {
			if time.Now().Before(expiry) {
				continue
			}
			connectionMutex.Lock()
			delete(RestrictedUntil, addr)
			connectionMutex.Unlock()

		}

		// rs.PrintPeerInfo(uint32(randomIndex))
		go handlePeerConnection(addr, uint32(randomIndex), req, pieceMgr)
		time.Sleep(10 * time.Millisecond)
	}
}

func BroadcastHave(pieceIndex uint32) {
	connectionMutex.RLock()
	connections := make([]*peer_protocol.PeerConn, 0, len(ActivePeerConnections))
	for _, conn := range ActivePeerConnections {
		connections = append(connections, conn)
	}
	connectionMutex.RUnlock()

	if len(connections) == 0 {
		return
	}

	havePayload := make([]byte, 4)
	binary.BigEndian.PutUint32(havePayload, pieceIndex)
	haveMsg := peer_protocol.NewMessageWP(5, peer_protocol.Have, havePayload)
	serialized := haveMsg.Serialize()

	for _, conn := range connections {
		go func(c *peer_protocol.PeerConn) {
			if c.Conn == nil {
				return
			}

			c.Conn.SetWriteDeadline(time.Now().Add(time.Second * 2))

			c.Conn.Write(serialized)
		}(conn)
	}

	fmt.Printf("*** broadcast have(%d) to %d peers\n", pieceIndex, len(connections))
}

func findUnusedPeerIndex(peers []*tracker.Peer) int {
	maxAttempts := len(peers) * 2

	for range maxAttempts {
		randomIndex := rand.IntN(len(peers))
		connectionMutex.RLock()
		_, inUse := ActivePeerConnections[uint32(randomIndex)]
		connectionMutex.RUnlock()
		if !inUse {
			return randomIndex
		}
	}

	return -1
}

func handlePeerConnection(peerAddr string, peerIdx uint32, req *tracker.Request, pieceMgr *peer_protocol.PieceManager) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic in peer %s: %v", peerAddr, r)
		}

		connectionMutex.Lock()
		delete(ActivePeerConnections, peerIdx)
		connectionMutex.Unlock()
	}()

	err := peer_protocol.HandleConnection(
		peerAddr,
		peerIdx,
		req,
		pieceMgr,
		func(conn *peer_protocol.PeerConn) {
			// conn.Conn.SetDeadline(time.Now().Add(time.Second * 5))

			connectionMutex.Lock()
			ActivePeerConnections[peerIdx] = conn
			connectionMutex.Unlock()
		},
		BroadcastHave,
	)

	if err != nil {
		fmt.Printf("peer disconnected: %v\n", err)
		connectionMutex.Lock()
		RestrictedUntil[peerAddr] = time.Now().Add(time.Minute * 1)
		connectionMutex.Unlock()
	}
}

func PrintTorResponseInfo(tor *torrent.Torrent, rs *tracker.Response) {
	tor.PrintMetadata()
	fmt.Printf("Peers: %d\n", len(rs.Peers))
	fmt.Printf("Interval: %ds\n", rs.Interval)
	fmt.Printf("Seeders: %d, Leechers: %d\n", rs.Seeders, rs.Leechers)
}
