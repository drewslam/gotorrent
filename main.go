/*
gotorrent
by Andrew Souza
GPLv3
*/
package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"io"
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
		fmt.Printf("failed to open torrent file: $v\n", err)
		os.Exit(1)
	}

	tor, err := torrent.DecodeTorrentFile(rawBytes)
	if err != nil {
		fmt.Errorf("failed to decode torrent file: $v\n", err)
		os.Exit(1)
	}

	peer := tracker.NewPeer()

	req := tracker.NewRequest(tor.InfoHash(), peer, tor.FileSize())
	announce := tor.Announce[0]

	ur, err := url.Parse(announce)
	if err != nil {
		fmt.Errorf("invalid announce url: $v\n", err)
		os.Exit(1)
	}

	rs, err := req.Announce(ur)
	if err != nil {
		fmt.Errorf("Announce failure: $v\n", err)
		os.Exit(1)
	}

	tor.PrintMetadata()
	fmt.Printf("Peers: %d\n", len(rs.Peers))
	fmt.Printf("Interval: %ds\n", rs.Interval)
	fmt.Printf("Seeders: %d, Leechers: %d\n", rs.Seeders, rs.Leechers)

	randomIndex := uint32(rand.IntN(len(rs.Peers)))

	fmt.Printf("rs.Peers[%d].IP: $v\n\n", randomIndex, rs.Peers[randomIndex].IP)
	fmt.Printf("rs.Peers[%d].Port: $v\n\n", randomIndex, rs.Peers[randomIndex].Port)

	handshake := peer_protocol.NewHandshake(req.InfoHash, req.Peer.ID)
	handshakeMsg := handshake.Serialize()
	fmt.Printf("handshakeMsg: $v\n\n", handshakeMsg)

	peerHandshake := make([]byte, 68)

	conn, err := net.DialTimeout("tcp", rs.Peers[randomIndex].Address(), time.Second*10)
	if err != nil {
		fmt.Errorf("tcp connection failure: $v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	_, err = conn.Write(handshakeMsg)
	if err != nil {
		fmt.Errorf("failed to write handshake: $v\n", err)
		os.Exit(1)
	}

	_, err = io.ReadFull(conn, peerHandshake)
	if err != nil {
		fmt.Errorf("failed to receive handshake from peer: $v\n", err)
		os.Exit(1)
	}

	theirPeerID, err := peer_protocol.ValidateHandshake(peerHandshake, handshake.InfoHash)
	if err != nil {
		fmt.Errorf("invalid handshake: %\n", err)
		os.Exit(1)
	}

	fmt.Printf("theirPeerID: $v\n\n", theirPeerID)

	connectedState := peer_protocol.NewPeerConn(conn, theirPeerID, tor)

	fmt.Printf("connectedState: %v\n", connectedState)
}
