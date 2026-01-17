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

type PeerConn struct {
	Conn   net.Conn
	PeerID [20]byte

	ClientState *PeerState
	PeerState   *PeerState

	NumPieces  uint32
	Bitfield   []byte
	HavePieces []byte
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
	numPieces := uint32(len(tor.Info.Pieces))
	bitfieldSize := (numPieces + 7) / 8

	return &PeerConn{
		Conn:        conn,
		PeerID:      peerID,
		ClientState: NewPeerState(),
		PeerState:   NewPeerState(),
		NumPieces:   numPieces,
		Bitfield:    make([]byte, bitfieldSize),
		HavePieces:  make([]byte, bitfieldSize),
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

	if lenPrefix != 0 {
		msgBuf := make([]byte, lenPrefix)
		_, err = io.ReadFull(p.Conn, msgBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to parse message body: %w", err)
		}

		fmt.Printf("msg received: %v\n", msgBuf)

		msgIndex := MsgIndex(msgBuf[0])

		switch msgIndex {
		case Choke:
			p.PeerState.Choking = true
		case Unchoke:
			p.PeerState.Choking = false
			// select appropriate piece index
			pieceIndex := -1
			for i := range p.NumPieces {
				if p.PeerHasPiece(i) && !p.WeHavePiece(i) {
					pieceIndex = int(i)
					break
				}
			}
			if pieceIndex == -1 {
				return nil, nil
			}
			// format request message
			return p.PrepareRequest(uint32(pieceIndex), 0)
		case Interested:
			p.PeerState.Interested = true
		case NotInterested:
			p.PeerState.Interested = false
		case Have:
			fmt.Printf("have\n")
		case Bitfield:
			p.Bitfield = msgBuf[1:]
			p.ClientState.Interested = true
			return &Message{
				Length: 1,
				ID:     Interested,
			}, nil
		case Request:
			fmt.Printf("request\n")
		case Piece:
			pieceIndex := binary.BigEndian.Uint32(msgBuf[1:5])
			offset := binary.BigEndian.Uint32(msgBuf[5:9])
			blockData := msgBuf[9:]

			fmt.Printf("piece: %d\noffset: %d\n%d bytes\n", pieceIndex, offset, len(blockData))
			// TODO verify hash and save data
		case Cancel:
			fmt.Printf("cancel\n")
		default:
			fmt.Printf("unrecognized message ID: %d\n", msgIndex)
		}

	}

	return nil, nil
}

func (p *PeerConn) PrepareRequest(index uint32, offset uint32) (*Message, error) {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], offset)
	binary.BigEndian.PutUint32(payload[8:12], MaxBlockSize)

	return &Message{
		Length: 13,
		ID: Request,
		Payload: payload,
	}, nil
}

func (p *PeerConn) PeerHasPiece(index uint32) bool {
	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	if byteIndex >= uint32(len(p.Bitfield)) {
		return false
	}
	return (p.Bitfield[byteIndex] & (1 << bitIndex)) != 0
}

func (p *PeerConn) WeHavePiece(index uint32) bool {
	byteIndex := index / 8
	bitIndex := (7 - (index % 8))
	if byteIndex >= uint32(len(p.HavePieces)) {
		return false
	}
	return (p.HavePieces[byteIndex] & (1 << bitIndex)) != 0
}
