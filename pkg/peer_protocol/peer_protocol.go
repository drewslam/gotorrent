/*
 *
 * title: gotorrent peer_protocol
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"fmt"
	"net"
	"time"

	"github.com/drewslam/gotorrent/pkg/tracker"
)

type MsgIndex uint8

const (
	Choke MsgIndex = iota
	Unchoke
	Interested
	NotInterested
	Have
	Bitfield
	Request
	Piece
	Cancel
	MaxMsgIndex = Cancel
)

const MaxBlockSize uint32 = 16384

type PeerState struct {
	Choking    bool
	Interested bool
}

func NewPeerState() *PeerState {
	return &PeerState{
		Choking:    true,
		Interested: false,
	}
}

func (pm *PieceManager) HandleConnection(peerAddr string, peerIdx uint32, req *tracker.Request) {
	handshake := NewHandshake(req.InfoHash, req.Peer.ID)

	conn, err := net.DialTimeout("tcp", peerAddr, time.Second*10)
	if err != nil {
		fmt.Printf("tcp connection failure: %v\n", err)
		return
	}
	defer conn.Close()

	theirPeerID, err := handshake.FetchPeer(conn)
	if err != nil {
		fmt.Printf("peer %d handshake failed: %v\n", peerIdx, err)
		return
	}

	connectedState := NewPeerConn(conn, theirPeerID, pm)

	// keep alive
	ticker := time.NewTicker(time.Minute * 2)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			keepAlive := &Message{Length: 0}
			if _, err := conn.Write(keepAlive.Serialize()); err != nil {
				return
			}
		}
	}()

	for {
		msg, err := connectedState.ReadMsg()
		if err != nil {
			fmt.Printf("peer %d read error: %v\n", peerIdx, err)
			return
		}

		fmt.Printf("msg %s received from peer %d\n", msg.ID.String(), peerIdx)

		err = connectedState.WriteMsgResponse(msg)
		if err != nil {
			fmt.Printf("peer %d write error: %v\n", peerIdx, err)
			return
		}
	}
}

func PeerHasPiece(bitfield []byte, index uint32) bool {
	return isBitInBitfield(index, bitfield)
}

func isBitInBitfield(index uint32, bf []byte) bool {
	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	if byteIndex >= uint32(len(bf)) {
		return false
	}
	return (bf[byteIndex] & (1 << bitIndex)) != 0
}
