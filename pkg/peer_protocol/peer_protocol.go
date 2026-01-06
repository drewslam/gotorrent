/*
 *
 * title: gotorrent peer_protocol
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/drewslam/gotorrent/pkg/torrent"
)

type MsgVal uint8

const (
	Choke MsgVal = iota
	Unchoke
	Interested
	NotInterested
	Have
	Bitfield
	Request
	Piece
	Cancel
	MaxMsgVal = Cancel
)

type PeerState struct {
	Choking    bool
	Interested bool
}

type Handshake struct {
	Pstrlen  byte
	Pstr     [19]byte
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}

type Message struct {
	Length  uint32
	ID      MsgVal
	Payload []byte
}

type PeerConn struct {
	Conn net.Conn
	PeerID [20]byte

	ClientState *PeerState
	PeerState   *PeerState

	Bitfield []byte
}

func NewPeerState() *PeerState {
	return &PeerState{
		Choking:    true,
		Interested: false,
	}
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

func NewPeerConn(conn net.Conn, peerID [20]byte, tor *torrent.Torrent) *PeerConn {
	numPieces := len(tor.Info.Pieces)
	bitfieldSize := (numPieces + 7) / 8

	return &PeerConn{
		Conn:        conn,
		PeerID:      peerID,
		ClientState: NewPeerState(),
		PeerState:   NewPeerState(),
		Bitfield:    make([]byte, bitfieldSize),
	}
}

func (h *Handshake) Serialize() []byte {
	buffer := bytes.NewBuffer([]byte{h.Pstrlen})
	buffer.Write(h.Pstr[:])
	buffer.Write(h.Reserved[:])
	buffer.Write(h.InfoHash[:])
	buffer.Write(h.PeerID[:])
	return buffer.Bytes()
}

func (m *Message) Serialize() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, m.Length)

	if m.Length == 0 {
		return buffer.Bytes()
	}

	buffer.WriteByte(byte(m.ID))
	if m.Payload != nil {
		buffer.Write(m.Payload)
	}
	return buffer.Bytes()
}

func ValidateHandshake(input []byte, expectedInfoHash [20]byte) ([20]byte, error) {
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
