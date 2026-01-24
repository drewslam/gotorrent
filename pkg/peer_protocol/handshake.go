/*
 *
 * title: gotorrent peer_protocol handshake
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

type Handshake struct {
	Pstrlen  byte
	Pstr     [19]byte
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}

func NewHandshake(info [20]byte, peerID [20]byte) *Handshake {
	pstr := [19]byte{}
	copy(pstr[:], "BitTorrent protocol")
	return &Handshake{
		Pstrlen:  19,
		Pstr:     pstr,
		Reserved: [8]byte{0},
		InfoHash: info,
		PeerID:   peerID,
	}
}

func (h *Handshake) FetchPeer(conn net.Conn) ([20]byte, error) {
	peerHandshake := make([]byte, 68)
	_, err := conn.Write(h.serialize())
	if err != nil {
		return [20]byte{}, fmt.Errorf("failed to write handshake: %v", err)
	}

	_, err = io.ReadFull(conn, peerHandshake)
	if err != nil {
		return [20]byte{}, fmt.Errorf("failed to receive handshake from peer: %v", err)
	}

	theirPeerID, err := validateHandshake(peerHandshake, h.InfoHash)
	if err != nil {
		return [20]byte{}, fmt.Errorf("invalid handshake: %v", err)
	}

	return theirPeerID, nil
}

func (h *Handshake) serialize() []byte {
	buffer := bytes.NewBuffer([]byte{h.Pstrlen})
	buffer.Write(h.Pstr[:])
	buffer.Write(h.Reserved[:])
	buffer.Write(h.InfoHash[:])
	buffer.Write(h.PeerID[:])
	return buffer.Bytes()
}

func validateHandshake(input []byte, expectedInfoHash [20]byte) ([20]byte, error) {
	if len(input) != 68 {
		return [20]byte{0}, fmt.Errorf("incorrect length handshake")
	}

	if input[0] != 0x13 {
		return [20]byte{0}, fmt.Errorf("invalid length prefix")
	}
	if string(input[1:20]) != "BitTorrent protocol" {
		return [20]byte{0}, fmt.Errorf("invalid protocol message")
	}

	var infoHash [20]byte
	copy(infoHash[:], input[28:48])
	if infoHash != expectedInfoHash {
		return [20]byte{0}, fmt.Errorf("info hash mismatch")
	}

	var peerID [20]byte
	copy(peerID[:], input[48:])

	return peerID, nil
}
