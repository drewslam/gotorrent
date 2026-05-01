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

func HandleConnection(peerAddr string, peerIdx uint32, req *tracker.Request, pm *PieceManager, onConnect func(*PeerConn), onHave func(uint32)) error {
	handshake := NewHandshake(req.InfoHash, req.Peer.ID)

	conn, err := net.DialTimeout("tcp", peerAddr, time.Second*10)
	if err != nil {
		return fmt.Errorf("tcp connection failure")
	}
	defer conn.Close()

	theirPeerID, err := handshake.FetchPeer(conn)
	if err != nil {
		return fmt.Errorf("peer %d handshake failed: %v", peerIdx, err)
	}

	bf := NewMessageWP(uint32(1+len(pm.DataMgr.Completed)), Bitfield, pm.DataMgr.Completed)
	conn.Write(bf.Serialize())

	connectedState := NewPeerConn(conn, theirPeerID, pm)
	defer pm.RemovePeerBitfield(connectedState.Bitfield)

	onConnect(connectedState)

	// keep alive
	ticker_keepalive := time.NewTicker(time.Minute * 2)
	defer ticker_keepalive.Stop()

	go func() {
		for range ticker_keepalive.C {
			keepAlive := &Message{Length: 0}
			if _, err := conn.Write(keepAlive.Serialize()); err != nil {
				return
			}
		}
	}()

	// check completion
	ticker_completion := time.NewTicker(time.Minute * 20)
	defer ticker_completion.Stop()

	go func() {
		for range ticker_completion.C {
				pm.PrintMissingPieces()
		}
	}()

	// release stalled blocks
	ticker_requested := time.NewTicker(time.Second * 30)
	defer ticker_requested.Stop()

	go func() {
		for range ticker_requested.C {
			pm.ReleaseTimedOutBlocks(time.Minute * 2)
		}
	}()

	for {
		msg, err := connectedState.ReadMsg()
		if err != nil {
			return fmt.Errorf("peer %d read error: %v", peerIdx, err)
		}

		fmt.Printf("msg %s received from peer %d\n", msg.ID.String(), peerIdx)

		if msg != nil {
			if err := connectedState.WriteMsgResponse(msg, onHave); err != nil {
				return fmt.Errorf("peer %d write error: %v", peerIdx, err)
			}
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
