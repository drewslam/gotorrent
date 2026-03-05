/*
 *
 * title: gotorrent peer_protocol peerconn
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"crypto/sha1"
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

func (p *PeerConn) WriteMsgResponse(msg *Message, onHave func(uint32)) error {
	var newMsg *Message
	var err error

	switch msg.ID {
	case Unchoke:
		fmt.Printf("peer unchoked. bitfield length: %d\n", len(p.Bitfield))
		if len(p.Bitfield) == 0 {
			fmt.Printf("   WARNING: peer bitfield is empty\n")
		}

		requestIndex, requestOffset, requestLength, ok := p.PieceMgr.HandleUnchoke(p.Bitfield)
		if ok {
			newMsg, err = p.PieceMgr.prepareRequest(requestIndex, requestLength, requestOffset)
			if err != nil {
				return fmt.Errorf("PrepareRequest failure: %w", err)
			}
		}
	case Bitfield:
		// amInterested = ParseBitfield(p.Bitfield)
		p.ClientState.Interested = true
		newMsg = NewMessageNP(Interested)
	case Piece:
		pieceIndex, isComplete := p.PieceMgr.HandlePieceMessage(msg)

		if isComplete {
			buf := p.PieceMgr.DataMgr.AssembleAndClear(pieceIndex)
			verified := p.PieceMgr.DataMgr.VerifyPiece(pieceIndex, buf)
			if verified {
				fmt.Printf("*** piece %d verified and written\n", pieceIndex)
				err = p.PieceMgr.Storage.WritePiece(pieceIndex, buf)
				if err != nil {
					return fmt.Errorf("WritePiece failure: %w", err)
				}
				onHave(pieceIndex)
			} else {
				fmt.Printf("!!! piece %d FAILED verification\n", pieceIndex)
				fmt.Printf("    expected: %x\n", p.PieceMgr.DataMgr.PiecesHash[pieceIndex])
				fmt.Printf("    got: %x\n", sha1.Sum(buf))
				fmt.Printf("    piece size: %d bytes\n", len(buf))
			}

			newMsg, err = p.PieceMgr.FinishPiece(pieceIndex, p.Bitfield, verified)
			if err != nil {
				return fmt.Errorf("FinishPiece failure: %w", err)
			}
		} else {
			nextOffset, nextLength, ok := p.PieceMgr.GetNextBlock(pieceIndex)
			if !ok {
				// return fmt.Errorf("piece state missing for index %d", pieceIndex)
				break
			}

			newMsg, err = p.PieceMgr.prepareRequest(pieceIndex, nextLength, nextOffset)
			if err != nil {
				return fmt.Errorf("PrepareRequest failure: %w", err)
			}
		}
	case Request:
		if len(msg.Payload) < 12 {
			return fmt.Errorf("invalid request message")
		}

		rindex := binary.BigEndian.Uint32(msg.Payload[0:4])
		roffset := binary.BigEndian.Uint32(msg.Payload[4:8])
		rlength := binary.BigEndian.Uint32(msg.Payload[8:12])

		fmt.Printf("peer requested: piece=%d offset=%d length=%d\n", rindex, roffset, rlength)

		if !isBitInBitfield(rindex, p.PieceMgr.DataMgr.Completed) {
			fmt.Printf("   don't have piece %d yet, can't upload\n", rindex)
			break
		}

		piece, err := p.PieceMgr.Storage.ReadPiece(rindex)
		if err != nil {
			fmt.Printf("    failed to read piece %d: %v\n", rindex, err)
			break
		}

		if roffset+rlength > uint32(len(piece)) {
			fmt.Printf("    invalid request: offset+length exceeds piece size\n")
			break
		}

		blockData := piece[roffset:roffset+rlength]

		payload := make([]byte, 8+len(blockData))
		binary.BigEndian.PutUint32(payload[0:4], rindex)
		binary.BigEndian.PutUint32(payload[4:8], roffset)
		copy(payload[8:], blockData)

		newMsg = NewMessageWP(uint32(1 + len(payload)), Piece, payload)
	}

	if newMsg != nil {
		fmt.Printf("newMsg: %v\n", newMsg)
		_, err = p.Conn.Write(newMsg.Serialize())
		if err != nil {
			return fmt.Errorf("p.Conn.Write failure: %w", err)
		}
	}

	return nil
}
