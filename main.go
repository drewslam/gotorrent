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
	"time"

	"net/url"
	"os"

	"github.com/drewslam/gotorrent/pkg/connection_manager"
	"github.com/drewslam/gotorrent/pkg/peer_protocol"
	"github.com/drewslam/gotorrent/pkg/storage"
	"github.com/drewslam/gotorrent/pkg/torrent"
	"github.com/drewslam/gotorrent/pkg/tracker"
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

	storage, err := storage.NewFileStorage(tor, tor.Info.Name)
	if err != nil {
		log.Fatalf("NewFileStorage failure: %v", err)
	}

	pieceMgr := peer_protocol.NewPieceManager(tor, storage)

	if err := storage.Allocate(); err != nil {
		log.Fatalf("allocation failure: %v", err)
	}

	pr := connection_manager.NewPeerRegistry(req, pieceMgr)
	go pr.ClearExpiredBans()
	go pr.ManagePeerConnections(connection_manager.MaxPeerConnections)

	for {
		rs, err := AnnounceToTrackers(tor.Announce, req)
		if err != nil {
			log.Print("announceToTrackers failure: %w", err)
		}
		interval := time.Duration(rs.Interval) * time.Second

		PrintTorrentResponseInfo(tor, rs)

		pr.UpdatePeerList(rs.Peers)

		time.Sleep(interval)
	}

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

func PrintTorrentResponseInfo(tor *torrent.Torrent, rs *tracker.Response) {
	tor.PrintMetadata()
	fmt.Printf("Peers: %d\n", len(rs.Peers))
	fmt.Printf("Interval: %ds\n", rs.Interval)
	fmt.Printf("Seeders: %d, Leechers: %d\n", rs.Seeders, rs.Leechers)
}
