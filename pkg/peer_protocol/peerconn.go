/*
 *
 * title: gotorrent peer_protocol peerconn
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type PeerConn struct {
	Conn   net.Conn
	PeerID [20]byte

	ClientState *PeerState
	PeerState   *PeerState

	Bitfield []byte

	PieceMgr *PieceManager
}

func NewPeerConn(conn net.Conn, peerID [20]byte, pm *PieceManager) *PeerConn {
	bitfieldSize := (pm.NumPieces + 7) / 8

	return &PeerConn{
		Conn:        conn,
		PeerID:      peerID,
		ClientState: NewPeerState(),
		PeerState:   NewPeerState(),
		Bitfield:    make([]byte, bitfieldSize),
		PieceMgr:    pm,
	}
}

func (p *PeerConn) ReadMsg() (*Message, error) {
	lenBuffer := make([]byte, 4)
	_, err := io.ReadFull(p.Conn, lenBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	lenPrefix := binary.BigEndian.Uint32(lenBuffer[:])
	if lenPrefix == 0 {
		return &Message{Length: 0}, nil
	}

	msgBuf := make([]byte, lenPrefix)
	_, err = io.ReadFull(p.Conn, msgBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message body: %w", err)
	}

	msgIndex := MsgIndex(msgBuf[0])

	switch msgIndex {
	case Choke:
		p.PeerState.Choking = true
	case Unchoke:
		p.PeerState.Choking = false
	case Interested:
		p.PeerState.Interested = true
	case NotInterested:
		p.PeerState.Interested = false
	case Bitfield:
		p.Bitfield = msgBuf[1:]
	}

	return NewMessageWP(lenPrefix, msgIndex, msgBuf[1:]), nil
}

func (p *PeerConn) WriteMsgResponse(msg *Message) error {
	var newMsg *Message
	var err error

	switch msg.ID {
	case Unchoke:
		pieceIndex, ok := p.PieceMgr.SelectPiece(p.Bitfield)
		if !ok {
			return nil
		}

		newMsg, err = p.PrepareRequest(uint32(pieceIndex), 0)
		if err != nil {
			return fmt.Errorf("PrepareRequest failure: %w", err)
		}
	case Bitfield:
		p.ClientState.Interested = true
		newMsg = NewMessageNP(Interested)
	case Piece:
		pieceIndex := binary.BigEndian.Uint32(msg.Payload[0:4])
		offset := binary.BigEndian.Uint32(msg.Payload[4:8])
		blockData := msg.Payload[8:]

		fmt.Printf("received block: piece=%d offset=%d size=%d bytes\n", pieceIndex, offset, len(blockData))
		// TODO verify hash and save data
	}

	if newMsg != nil {
		_, err = p.Conn.Write(newMsg.Serialize())
		if err != nil {
			return fmt.Errorf("p.Conn.Write failure: %w", err)
		}
	}

	return nil
}

func (p *PeerConn) PrepareRequest(index uint32, offset uint32) (*Message, error) {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], offset)
	binary.BigEndian.PutUint32(payload[8:12], MaxBlockSize)

	return NewMessageWP(13, Request, payload), nil
}
