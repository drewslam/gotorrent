/*
gotorrent
by Andrew Souza
GPLv3
*/
package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/drewslam/gotorrent/pkg/torrent"
	"github.com/drewslam/gotorrent/pkg/tracker"
)

func main() {
	args := os.Args
	rawBytes, err := os.ReadFile(args[1])
	if err != nil {
		log.Fatalf("failed to open torrent file: %v", err)
	}

	tor, err := torrent.DecodeTorrentFile(rawBytes)
	if err != nil {
		log.Fatalf("failed to decode torrent file: %v", err)
	}

	peer := tracker.NewPeer()

	req := tracker.NewRequest(tor.InfoHash(), peer, tor.FileSize())
	announce := tor.Announce[0]

	ur, err := url.Parse(announce)
	if err != nil {
		log.Fatalf("invalid announce url: %v", err)
	}

	rs, err := req.Announce(ur)
	if err != nil {
		log.Fatalf("Announce failure: %v", err)
	}

	tor.PrintMetadata()
	fmt.Printf("Peers: %d\n", len(rs.Peers))
	fmt.Printf("Interval: %ds\n", rs.Interval)
	fmt.Printf("Seeders: %d, Leechers: %d\n", rs.Seeders, rs.Leechers)
}
