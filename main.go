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

	"github.com/drewslam/gotorrent/pkg/bcodec"
	"github.com/drewslam/gotorrent/pkg/torrent"
	"github.com/drewslam/gotorrent/pkg/tracker"
)

func DecodeTorrentFile(file []byte) (*torrent.Torrent, error) {
	decoder, err := bcodec.NewBDecoder(file, false, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %v", err)
	}
	decoded, err := decoder.DecodeDict()
	if err != nil {
		return nil, fmt.Errorf("failed to decode torrent data: %v", err)
	}
	tor, err := torrent.ParseMetadata(decoded, file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse torrent metadata: %v", err)
	}
	return tor, nil
}

func main() {
	args := os.Args
	rawBytes, err := os.ReadFile(args[1])
	if err != nil {
		log.Fatalf("failed to open torrent file: %v", err)
	}

	tor, err := DecodeTorrentFile(rawBytes)
	if err != nil {
		log.Fatalf("failed to decode torrent file: %v", err)
	}

	fileSize := tor.FileSize()
	sum := tor.InfoHash()

	peer := tracker.NewPeer()

	req := tracker.NewRequest(sum, peer, fileSize)
	announce := tor.Announce[0]

	// rs, err := req.Announce(announce)
	ur, err := url.Parse(announce)
	if err != nil {
		log.Fatalf("invalid announce url: %v", err)
	}

	var rs *tracker.Response
	switch ur.Scheme {
	case "http", "https":
		rs, err = req.HttpAnnounce(ur)
		if err != nil {
			log.Fatalf("failed to receive http response: %v", err)
		}
	case "udp":
		rs, err = req.UdpAnnounce(ur)
		if err != nil {
			log.Fatalf("failed to receive udp response: %v", err)
		}

	case "":
		fmt.Println("no announce scheme detected")
	default:
		fmt.Printf("unsupported scheme: %s\n", ur.Scheme)
	}

	tor.PrintMetadata()
	fmt.Printf("Peers: %d\n", len(rs.PeerDict))
	fmt.Printf("Interval: %ds\n", rs.Interval)
	fmt.Printf("Seeders: %d, Leechers :%d\n", rs.UdpResponse.Seeders, rs.UdpResponse.Leechers)
}
