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
	"math/rand/v2"
	"time"

	"net"
	"net/url"
	"os"

	"github.com/drewslam/gotorrent/pkg/peer_protocol"
	"github.com/drewslam/gotorrent/pkg/torrent"
	"github.com/drewslam/gotorrent/pkg/tracker"
)

const MaxPeerConnections uint32 = 5

func main() {
	args := os.Args
	rawBytes, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Printf("failed to open torrent file: %v\n", err)
		os.Exit(1)
	}

	tor, err := torrent.DecodeTorrentFile(rawBytes)
	if err != nil {
		fmt.Printf("failed to decode torrent file: %v\n", err)
		os.Exit(1)
	}

	peer := tracker.NewPeer()

	req := tracker.NewRequest(tor.InfoHash(), peer, tor.FileSize())
	announce := tor.Announce[0]

	ur, err := url.Parse(announce)
	if err != nil {
		fmt.Printf("invalid announce url: %v\n", err)
	}

	rs, err := req.Announce(ur)
	if err != nil {
		fmt.Printf("Announce failure: %v\n", err)
	}

	if len(rs.Peers) == 0 && len(tor.Announce) > 1 {
		for _, announce := range tor.Announce[1:] {
			ur, err = url.Parse(announce)
			if err != nil {
				fmt.Printf("invalid announce url: %v\n", err)
				continue
			}

			rs, err = req.Announce(ur)
			if err != nil {
				fmt.Printf("Announce failure: %v\n", err)
				continue
			}

			if len(rs.Peers) > 0 {
				break
			}
		}
	}

	if len(rs.Peers) == 0 {
		fmt.Printf("no peers found\n")
		os.Exit(1)
	}

	tor.PrintMetadata()
	fmt.Printf("Peers: %d\n", len(rs.Peers))
	fmt.Printf("Interval: %ds\n", rs.Interval)
	fmt.Printf("Seeders: %d, Leechers: %d\n", rs.Seeders, rs.Leechers)

	pieceMgr := peer_protocol.NewPieceManager(tor)

	for range MaxPeerConnections {
		randomIndex := uint32(rand.IntN(len(rs.Peers)))
		rs.PrintPeerInfo(randomIndex)
		go func(peerAddr string, peerIdx uint32) {
			HandleConnection(peerAddr, peerIdx, req, pieceMgr)
		}(rs.Peers[randomIndex].Address(), randomIndex)
	}

	select {}
}

func HandleConnection(peerAddr string, peerIdx uint32, req *tracker.Request, pieceMgr *peer_protocol.PieceManager) {
	handshake := peer_protocol.NewHandshake(req.InfoHash, req.Peer.ID)

	conn, err := net.DialTimeout("tcp", peerAddr, time.Second*10)
	if err != nil {
		fmt.Printf("tcp connection failure: %v\n", err)
		return
	}
	defer conn.Close()

	theirPeerID, err := handshake.FetchPeer(conn)
	if err != nil {
		fmt.Printf("peer %d handshake failed: %v\n", peerIdx, err)
	}

	connectedState := peer_protocol.NewPeerConn(conn, theirPeerID, pieceMgr)

	// keep alive
	ticker := time.NewTicker(time.Minute * 2)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			keepAlive := &peer_protocol.Message{Length: 0}
			if _, err := conn.Write(keepAlive.Serialize()); err != nil {
				return
			}
		}
	}()

	for {
		msg, err := connectedState.ReadMsg()
		if err != nil {
			fmt.Printf("peer %d read error: %v\n", peerIdx, err)
			return
		}

		err = connectedState.WriteMsgResponse(msg)
		if err != nil {
			fmt.Printf("peer %d write error: %v\n", peerIdx, err)
			return
		}
	}
}
