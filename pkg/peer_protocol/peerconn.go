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
	bitfieldSize := (pm.DataMgr.NumPieces + 7) / 8

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
		requestIndex, requestOffset, _, ok := p.PieceMgr.HandleUnchoke(p.Bitfield)
		if !ok {
			return fmt.Errorf("no block found")
		}

		newMsg, err = p.PieceMgr.PrepareRequest(requestIndex, requestOffset)
		if err != nil {
			return fmt.Errorf("PrepareRequest failure: %w", err)
		}
	case Bitfield:
		p.ClientState.Interested = true
		newMsg = NewMessageNP(Interested)
	case Piece:
		pieceIndex, isComplete := p.PieceMgr.HandlePieceMessage(msg)

		if isComplete {
			buf := p.PieceMgr.DataMgr.AssemblePiece(pieceIndex)
			verified := p.PieceMgr.DataMgr.VerifyPiece(pieceIndex, buf)
			if verified {
				fmt.Printf("*** Piece %d verified and written\n", pieceIndex)
				err = p.PieceMgr.Storage.WritePiece(pieceIndex, buf)
				if err != nil {
					return fmt.Errorf("WritePiece failure: %w", err)
				}
			}

			newMsg, err = p.PieceMgr.FinishPiece(pieceIndex, p.Bitfield, verified)
			if err != nil {
				return fmt.Errorf("FinishPiece failure: %w", err)
			}
		} else {
			nextOffset, _, ok := p.PieceMgr.GetNextBlock(pieceIndex)
			if !ok {
				return fmt.Errorf("piece state missing for index %d", pieceIndex)
			}

			newMsg, err = p.PieceMgr.PrepareRequest(pieceIndex, nextOffset)
			if err != nil {
				return fmt.Errorf("PrepareRequest failure: %w", err)
			}
		}
	}

	if newMsg != nil {
		_, err = p.Conn.Write(newMsg.Serialize())
		if err != nil {
			return fmt.Errorf("p.Conn.Write failure: %w", err)
		}
	}

	return nil
}
