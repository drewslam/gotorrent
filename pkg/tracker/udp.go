/*
 * title: gotorrent-tracker udp
 * author: Andrew Souza
 * GPLv3
 */
package tracker

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
)

type UDPTrackerConn struct {
	ConnID      uint64
	Transaction uint32
}

type AnnounceParams struct {
	InfoHash   [20]byte
	PeerID     [20]byte
	Downloaded uint64
	Left       uint64
	Uploaded   uint64
	Event      uint32
	IPOverride uint32
	Key        uint32
	NumWant    int32
	Port       uint16
}

type AnnounceResponse struct {
	Action        uint32
	TransactionID uint32
	Interval      uint32
	Leechers      uint32
	Seeders       uint32
	Peers         []*Peer
}

func (r *Request) udpAnnounce(announce *url.URL) (*Response, error) {
	remoteAddr, err := resolveRemoteAddr(announce)
	if err != nil {
		return nil, fmt.Errorf("resolveRemoteAddr failure: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial udp address: %v", err)
	}
	defer conn.Close()

	response, err := UDPConnect(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to udp network: %v", err)
	}

	announceTransactionId := NewUint32()

	event, err := r.Event.Uint32()
	if err != nil {
		return nil, fmt.Errorf("invalid event value")
	}

	params := &AnnounceParams{
		InfoHash:   r.InfoHash,
		PeerID:     r.Peer.ID,
		Downloaded: r.Downloaded,
		Left:       r.Left,
		Uploaded:   r.Uploaded,
		Event:      event,
		IPOverride: 0,
		Key:        r.Key,
		NumWant:    -1,
		Port:       r.Peer.Port,
	}

	announceRequest, err := BuildAnnounceRequest(response.ConnID, announceTransactionId, params)
	if err != nil {
		return nil, fmt.Errorf("BuildAnnounceRequest failure: %w", err)
	}

	_, err = conn.Write(announceRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to write to UDP server: %v", err)
	}

	responseBuffer := make([]byte, 1024)
	responseLen, err := conn.Read(responseBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read from UDP source: %v", err)
	}

	announceResponse, err := ParseAnnounceResponse(responseBuffer[:responseLen])
	if err != nil {
		return nil, fmt.Errorf("failed to decode announce response: %v", err)
	}

	if announceTransactionId != announceResponse.TransactionID {
		return nil, fmt.Errorf("mismatched transaction IDs")
	}

	return &Response{
		Interval: uint64(announceResponse.Interval),
		Peers:    announceResponse.Peers,
		Seeders:  announceResponse.Seeders,
		Leechers: announceResponse.Leechers,
	}, nil
}

func UDPConnect(conn *net.UDPConn) (*UDPTrackerConn, error) {
	trx := NewUint32()

	connReq := connectRequest(trx)

	if _, err := conn.Write(connReq); err != nil {
		return nil, fmt.Errorf("failed to write to UDP server: %v", err)
	}

	res := make([]byte, 16)
	if _, err := conn.Read(res); err != nil {
		return nil, fmt.Errorf("failed to read from UDP source: %v", err)
	}

	action := binary.BigEndian.Uint32(res[0:4])
	returnedTrx := binary.BigEndian.Uint32(res[4:8])
	connID := binary.BigEndian.Uint64(res[8:16])

	if action != 0 || returnedTrx != trx {
		return nil, fmt.Errorf("bad connection response")
	}

	return &UDPTrackerConn{ConnID: connID, Transaction: trx}, nil
}

func connectRequest(trx uint32) []byte {
	connReq := make([]byte, 0, 16)
	connReq = binary.BigEndian.AppendUint64(connReq, 0x41727101980)
	connReq = binary.BigEndian.AppendUint32(connReq, 0)
	connReq = binary.BigEndian.AppendUint32(connReq, trx)
	return connReq
}

func resolveRemoteAddr(announce *url.URL) (*net.UDPAddr, error) {
	remoteHost := announce.Hostname()
	remotePort := announce.Port()

	remoteAddr, err := net.ResolveUDPAddr("udp", remoteHost+":"+remotePort)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve udp address: %v", err)
	}

	return remoteAddr, nil
}

func BuildAnnounceRequest(connID uint64, trx uint32, p *AnnounceParams) ([]byte, error) {
	req := make([]byte, 0, 98)
	req = binary.BigEndian.AppendUint64(req, connID)
	req = binary.BigEndian.AppendUint32(req, 1)
	req = binary.BigEndian.AppendUint32(req, trx)
	req, err := binary.Append(req, binary.BigEndian, p.InfoHash)
	if err != nil {
		return nil, fmt.Errorf("failed to append info hash: %v", err)
	}
	req, err = binary.Append(req, binary.BigEndian, p.PeerID)
	if err != nil {
		return nil, fmt.Errorf("failed to append peer id: %v", err)
	}
	req = binary.BigEndian.AppendUint64(req, p.Downloaded)
	req = binary.BigEndian.AppendUint64(req, p.Left)
	req = binary.BigEndian.AppendUint64(req, p.Uploaded)
	req = binary.BigEndian.AppendUint32(req, 0)
	req = binary.BigEndian.AppendUint32(req, 0)
	req = binary.BigEndian.AppendUint32(req, p.Key)
	req, err = binary.Append(req, binary.BigEndian, p.NumWant)
	if err != nil {
		return nil, fmt.Errorf("failed to append p.NumWant: %v", err)
	}
	req = binary.BigEndian.AppendUint16(req, p.Port)
	return req, nil
}

func ParseAnnounceResponse(response []byte) (*AnnounceResponse, error) {
	if len(response) < 20 {
		return nil, fmt.Errorf("invalid response length: %d", len(response))
	}

	action := binary.BigEndian.Uint32(response[0:4])
	if action != 1 {
		return nil, fmt.Errorf("invalid action: %d", action)
	}

	transactionID := binary.BigEndian.Uint32(response[4:8])
	interval := binary.BigEndian.Uint32(response[8:12])
	leechers := binary.BigEndian.Uint32(response[12:16])
	seeders := binary.BigEndian.Uint32(response[16:20])

	peers := response[20:]
	numPeers := len(peers) / 6
	peerList := make([]*Peer, 0, numPeers)

	for i := range numPeers {
		offset := 6 * i
		ip := net.IPv4(peers[offset], peers[offset+1], peers[offset+2], peers[offset+3])
		port := binary.BigEndian.Uint16(peers[offset+4 : offset+6])

		peerList = append(peerList, NewPeer(ip, port))
	}

	return &AnnounceResponse{
		Action:        action,
		TransactionID: transactionID,
		Interval:      interval,
		Leechers:      leechers,
		Seeders:       seeders,
		Peers:         peerList,
	}, nil
}
