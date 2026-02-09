/*
 *
 * title: gotorrent peer_protocol message
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Message struct {
	Length  uint32
	ID      MsgIndex
	Payload []byte
}

func NewMessageNP(id MsgIndex) *Message {
	return &Message{
		Length: 1,
		ID:     id,
	}
}

func NewMessageWP(length uint32, id MsgIndex, payload []byte) *Message {
	return &Message{
		Length:  length,
		ID:      id,
		Payload: payload,
	}
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

func (m MsgIndex) String() string {
	names := [...]string{
		"Choke", "Unchoke", "Interested", "NotInterested",
		"Have", "Bitfield", "Request", "Piece", "Cancel",
	}

	if m <= MaxMsgIndex {
		return names[m]
	}

	return fmt.Sprintf("Unknown(%d)", m)
}
