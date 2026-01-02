/*
 *
 * title: gotorrent peer_protocol
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"net"

	"github.com/drewslam/gotorrent/pkg/torrent"
	"github.com/drewslam/gotorrent/pkg/tracker"
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
	Peer *tracker.Peer

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

func NewHandshake(req *tracker.Request) *Handshake {
	pstr := [19]byte{}
	copy(pstr[:], "BitTorrent protocol")
	return &Handshake{
		Pstrlen:  19,
		Pstr:     pstr,
		Reserved: [8]byte{0},
		InfoHash: req.InfoHash,
		PeerID:   req.Peer.ID,
	}
}

func NewPeerConn(conn net.Conn, peer *tracker.Peer, tor *torrent.Torrent) *PeerConn {
	numPieces := len(tor.Info.Pieces)
	bitfieldSize := (numPieces + 7) / 8

	return &PeerConn{
		Conn:        conn,
		Peer:        peer,
		ClientState: NewPeerState(),
		PeerState:   NewPeerState(),
		Bitfield:    make([]byte, bitfieldSize),
	}
}
