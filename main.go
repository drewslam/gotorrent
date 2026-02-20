/*
 *
 * title:   gotorrent
 * author:  Andrew Souza
 * license: GPLv3
 *
 */
package main

import (
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

const MaxPeerConnections uint32 = 50

var ActivePeerConnections = make(map[uint32]bool, MaxPeerConnections)
var connectionMutex sync.Mutex

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <torrent-file>\n", os.Args[0])
		os.Exit(1)
	}

	tor, err := loadTorrent(os.Args)
	if err != nil {
		log.Fatalf("loadTorrent failure: %v", err)
	}

	peer := tracker.NewPeer()
	req := tracker.NewRequest(tor.InfoHash(), peer, tor.FileSize())

	rs, err := announceToTrackers(tor.Announce, req)
	if err != nil {
		log.Fatalf("announceToTrackers failure: %v", err)
	}

	PrintTorResponseInfo(tor, rs)

	storage, err := storage.NewFileStorage(tor, tor.Info.Name)
	if err != nil {
		log.Fatalf("NewFileStorage failure: %v", err)
	}

	pieceMgr := peer_protocol.NewPieceManager(tor, storage)

	if err := storage.Allocate(); err != nil{
		log.Fatalf("allocation failure: %v", err)
	}

	connectToPeers(rs, req, pieceMgr)

	select {}
}

func loadTorrent(input []string) (*torrent.Torrent, error) {
	rawBytes, err := os.ReadFile(input[1])
	if err != nil {
		return nil, fmt.Errorf("failed to open torrent file: %v\n", err)
	}

	return torrent.DecodeTorrentFile(rawBytes)
}

func announceToTrackers(announceList []string, req *tracker.Request) (*tracker.Response, error) {
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

func connectToPeers(rs *tracker.Response, req *tracker.Request, pieceMgr *peer_protocol.PieceManager) {
	peers := rs.Peers
	numToConnect := min(int(MaxPeerConnections), len(peers))

	for len(ActivePeerConnections) < numToConnect {
		randomIndex := findUnusedPeerIndex(peers)
		if randomIndex == -1 {
			fmt.Printf("failed to find unique peer, stopping at %d connections\n", len(ActivePeerConnections))
			break
		}

		connectionMutex.Lock()
		ActivePeerConnections[uint32(randomIndex)] = true
		connectionMutex.Unlock()

		rs.PrintPeerInfo(uint32(randomIndex))

		go handlePeerConnection(rs.Peers[randomIndex].Address(), uint32(randomIndex), req, pieceMgr)

		time.Sleep(10 * time.Millisecond)
	}
}

func findUnusedPeerIndex(peers []*tracker.Peer) int {
	maxAttempts := len(peers) * 2

	for range maxAttempts {
		randomIndex := rand.IntN(len(peers))

		connectionMutex.Lock()
		inUse := ActivePeerConnections[uint32(randomIndex)]
		connectionMutex.Unlock()

		if !inUse {
			return randomIndex
		}
	}

	return -1
}

func handlePeerConnection(peerAddr string, peerIdx uint32, req *tracker.Request, pieceMgr *peer_protocol.PieceManager) {
	defer func() {
		connectionMutex.Lock()
		delete(ActivePeerConnections, peerIdx)
		connectionMutex.Unlock()
	}()

	err := peer_protocol.HandleConnection(peerAddr, peerIdx, req, pieceMgr)
	if err != nil {
		fmt.Printf("peer disconnected: %v\n", err)
		return
	}
}

func PrintTorResponseInfo(tor *torrent.Torrent, rs *tracker.Response) {
	tor.PrintMetadata()
	fmt.Printf("Peers: %d\n", len(rs.Peers))
	fmt.Printf("Interval: %ds\n", rs.Interval)
	fmt.Printf("Seeders: %d, Leechers: %d\n", rs.Seeders, rs.Leechers)
}
