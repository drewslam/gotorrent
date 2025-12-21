/*
gotorrent
by Andrew Souza
GPLv3
*/
package main

import (
	"fmt"
	"log"
	"net"
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
	sessionID := tracker.NewTransactionID()

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
		log.Fatalf("invalid announce url: %v", err)
	}

	var rs *tracker.Response
	switch ur.Scheme {
	case "http", "https":
		rs, err = req.FetchHttpResponse(announce)
		if err != nil {
			log.Fatalf("failed to receive http response: %v", err)
		}
	case "udp":
		// rs, err = req.FetchUdpResponse(ur)
		remoteHost := ur.Hostname()
		remotePort := ur.Port()
		remoteAddr, err := net.ResolveUDPAddr("udp", remoteHost+":"+remotePort)
		if err != nil {
			log.Fatalf("failed to resolve udp address: %v", err)
		}

		conn, err := net.DialUDP("udp", nil, remoteAddr)
		if err != nil {
			log.Fatalf("failed to dial udp address: %v", err)
		}
		defer conn.Close()

		response, err := tracker.UDPConnect(conn)
		if err != nil {
			log.Fatalf("failed to connect to udp network: %v", err)
		}

		annTrxID := tracker.NewTransactionID()

		udpAnn := &tracker.AnnounceParams{
			InfoHash:   sum,
			PeerID:     peer.ID,
			Downloaded: req.Downloaded,
			Left:       req.Left,
			Uploaded:   req.Uploaded,
			Event:      0,
			IPOverride: 0,
			Key:        sessionID,
			NumWant:    -1,
			Port:       uint16(req.Peer.Port),
		}

		announceRequest, err := tracker.BuildAnnounceRequest(response.ConnID, annTrxID, udpAnn)
		if err != nil {
			log.Fatal(err)
		}

		_, err = conn.Write(announceRequest)
		if err != nil {
			log.Fatalf("failed to write to UDP server: %v", err)
		}

		annRes := make([]byte, 1024)
		resLen, err := conn.Read(annRes)
		if err != nil {
			log.Fatalf("failed to read from UDP source: %v", err)
		}

		announceResponse, err := tracker.ParseAnnounceResponse(annRes[:resLen])
		if err != nil {
			log.Fatalf("failed to decode announce response: %v", err)
		}

		if annTrxID != announceResponse.TransactionID {
			log.Fatal("mismatched transaction ID")
		}


	case "":
		fmt.Println("no announce scheme detected")
	default:
		fmt.Printf("unsupported scheme: %s\n", ur.Scheme)
	}

	fmt.Printf("rs: %v\n", rs)
}
