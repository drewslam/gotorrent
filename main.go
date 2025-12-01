/*
gotorrent
by Andrew Souza
GPLv3
*/
package main

import (
	"fmt"
	"log"
	"os"
	"net/url"

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
	ur, err := url.Parse(announce)
	if err != nil {
		log.Fatalf("invalid transfer protocol: %v", err)
	}

	var rs *tracker.Response
	switch ur.Scheme {
	case "http", "https":
		rs, err = req.FetchHttpResponse(announce)
		if err != nil {
			log.Fatalf("failed to receive http response: %v", err)
		}
	case "udp":
		fmt.Println("udp to be implemented at a later date")
	case "":
		fmt.Println("no announce scheme detected")
	default:
		fmt.Printf("unsupported scheme: %s\n", ur.Scheme)
	}

	fmt.Printf("rs: %v\n", rs)
}
