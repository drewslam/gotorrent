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

	"net/url"
	"os"

	"github.com/drewslam/gotorrent/pkg/peer_protocol"
	"github.com/drewslam/gotorrent/pkg/torrent"
	"github.com/drewslam/gotorrent/pkg/tracker"
)

const MaxPeerConnections uint32 = 50
var ActivePeerConnections = make(map[uint32]bool, MaxPeerConnections)

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
		fmt.Printf("announce failure: %v\n", err)
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
				fmt.Printf("announce failure: %v\n", err)
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
	numToConnect := min(int(MaxPeerConnections), len(rs.Peers))

	for len(ActivePeerConnections) < numToConnect {
		var randomIndex uint32
		found := false
		maxAttempts := len(rs.Peers) * 2

		for range maxAttempts {
			randomIndex = uint32(rand.IntN(len(rs.Peers)))
			if !ActivePeerConnections[randomIndex] {
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("failed to find unique peer, stopping at %d connections\n", len(ActivePeerConnections))
			break
		}

		rs.PrintPeerInfo(randomIndex)
		ActivePeerConnections[randomIndex] = true

		go func(peerAddr string, peerIdx uint32) {
			peer_protocol.HandleConnection(peerAddr, peerIdx, req, pieceMgr)
		}(rs.Peers[randomIndex].Address(), randomIndex)
	}

	select {}
}
