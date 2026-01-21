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
	"io"
	"net"
	"sync"

	"github.com/drewslam/gotorrent/pkg/torrent"
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

type Handshake struct {
	Pstrlen  byte
	Pstr     [19]byte
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}

type Message struct {
	Length  uint32
	ID      MsgIndex
	Payload []byte
}

type PieceManager struct {
	NumPieces  uint32
	HavePieces []byte

	mu sync.RWMutex
}

type PeerConn struct {
	Conn   net.Conn
	PeerID [20]byte

	ClientState *PeerState
	PeerState   *PeerState

	Bitfield []byte

	PieceMgr *PieceManager
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

func NewPieceManager(tor *torrent.Torrent) *PieceManager {
	numPieces := uint32(len(tor.Info.Pieces))
	bitfieldSize := (numPieces + 7) / 8

	return &PieceManager{
		NumPieces:  numPieces,
		HavePieces: make([]byte, bitfieldSize),
	}
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

	return &Message{
		Length:  lenPrefix,
		ID:      msgIndex,
		Payload: msgBuf[1:],
	}, nil
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
		newMsg = &Message{Length: 1, ID: Interested}
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

func (pm *PieceManager) SelectPiece(bitfield []byte) (uint32, bool) {
	for i := range pm.NumPieces {
		if PeerHasPiece(bitfield, i) && !pm.WeHavePiece(i) {
			return i, true
		}
	}
	return 0, false
}

func (p *PeerConn) PrepareRequest(index uint32, offset uint32) (*Message, error) {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], offset)
	binary.BigEndian.PutUint32(payload[8:12], MaxBlockSize)

	return &Message{
		Length:  13,
		ID:      Request,
		Payload: payload,
	}, nil
}

func PeerHasPiece(bitfield []byte, index uint32) bool {
	return isBitInBitfield(index, bitfield)
}

func (pm *PieceManager) WeHavePiece(index uint32) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return isBitInBitfield(index, pm.HavePieces)
}

func isBitInBitfield(index uint32, bf []byte) bool {
	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	if byteIndex >= uint32(len(bf)) {
		return false
	}
	return (bf[byteIndex] & (1 << bitIndex)) != 0
}
