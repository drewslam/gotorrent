/*
gotorrent
by Andrew Souza
GPLv3
*/
package main

import (
	"log"
	"os"
	"strings"

	"github.com/drewslam/gotorrent/pkg/bcodec"
	"github.com/drewslam/gotorrent/pkg/torrent"
)

func main() {
	args := os.Args
	if !strings.Contains(args[1], ".torrent") {
		log.Fatalf("invalid file type passed: %s", args[1])
	}
	readBytes, err := os.ReadFile(args[1])
	if err != nil {
		log.Fatalf("failed to parse torrent file: %v", err)
	}
	dec, err := bcodec.NewBDecoder(readBytes, false, 0);
	if err != nil {
		log.Fatalf("failed to create decoder: %v", err)
	}
	decoded, err :=  dec.DecodeDict()
	if err != nil {
		log.Fatalf("failed to decode torrent data")
	}
	tor, err := torrent.ParseMetadata(decoded)
	if err != nil {
		log.Fatal(err)
	}
	tor.PrintMetadata()
}
