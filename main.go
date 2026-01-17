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

	randomIndex := uint32(rand.IntN(len(rs.Peers)))
	rs.PrintPeerInfo(randomIndex)

	handshake := peer_protocol.NewHandshake(req.InfoHash, req.Peer.ID)

	conn, err := net.DialTimeout("tcp", rs.Peers[randomIndex].Address(), time.Second*10)
	if err != nil {
		fmt.Printf("tcp connection failure: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	theirPeerID, err := handshake.FetchPeer(conn)

	connectedState := peer_protocol.NewPeerConn(conn, theirPeerID, tor)

	for {
		msg, err := connectedState.ReadMsg()
		if err != nil {
			fmt.Printf("ReadMsg failure: %v\n", err)
			break
		}

		fmt.Printf("msg sent: %v\n", msg)
	}
}
