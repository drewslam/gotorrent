/*
 *
 *  title: bencode-go
 *  author: Andrew Souza
 *  GPLv3
 *
 */
package bcodec

import (
	"fmt"
	"strconv"
)

type BDecoder struct {
	Data    []byte
	Pos     uint32
	Verbose bool
	Level   int
}

func NewBDecoder(data []byte, verbose bool, off uint32) (*BDecoder, error) {
	if data == nil {
		return nil, fmt.Errorf("data cannot be nil")
	}
	if off > uint32(len(data)) {
		return nil, fmt.Errorf("offset %d exceeds data length %d", off, len(data))
	}
	return &BDecoder{
		Data:    data,
		Pos:     off,
		Verbose: verbose,
		Level:   0,
	}, nil
}

func (b *BDecoder) Decode() (Node, error) {
	if b.Pos >= uint32(len(b.Data)) {
		return nil, fmt.Errorf("overflow error at position %d", b.Pos)
	}
	switch b.Data[b.Pos] {
	case 'd':
		return b.parseDict()
	case 'l':
		return b.parseList()
	case 'i':
		return b.parseInt()
	default:
		if b.Data[b.Pos] >= '0' && b.Data[b.Pos] <= '9' {
			return b.parseString()
		}
		return nil, fmt.Errorf("Illegal token at pos %d: %q", b.Pos, b.Data[b.Pos])
	}
}

func (b *BDecoder) DecodeList() (*BListNode, error) {
	n, err := b.Decode()
	if err != nil {
		return nil, err
	}
	if n.GetType() != LIST {
		return nil, fmt.Errorf("invalid type passed to DecodeList")
	}

	listNode, ok := n.(*BListNode)
	if !ok {
		return nil, fmt.Errorf("failed to cast to BListNode")
	}
	return listNode, nil
}

func (b *BDecoder) DecodeDict() (*BDictNode, error) {
	n, err := b.Decode()
	if err != nil {
		return nil, err
	}
	if n.GetType() != DICT {
		return nil, fmt.Errorf("invalid type passed to DecodeDict")
	}

	dictNode, ok := n.(*BDictNode)
	if !ok {
		return nil, fmt.Errorf("failed to cast to BDictNode")
	}
	return dictNode, nil
}

func (b *BDecoder) Position() uint32 {
	return b.Pos
}

func (b *BDecoder) parseString() (Node, error) {
	off := b.Pos
	start := b.Pos

	// Read until ':'
	for b.Pos < uint32(len(b.Data)) && b.Data[b.Pos] != ':' {
		if b.Data[b.Pos] < '0' || b.Data[b.Pos] > '9' {
			return nil, fmt.Errorf("invalid length byte at pos %d: %q", b.Pos, b.Data[b.Pos])
		}
		b.Pos++
	}

	if b.Pos >= uint32(len(b.Data)) || b.Data[b.Pos] != ':' {
		return nil, fmt.Errorf("expected ':' at pos %d", b.Pos)
	}

	// Parse string length
	lenStr := string(b.Data[start:b.Pos])
	strlen, err := strconv.Atoi(lenStr)
	if err != nil {
		return nil, fmt.Errorf("string length %d exceeds buffer at pos %d", strlen, int(b.Pos))
	}

	if strlen < 0 {
		return nil, fmt.Errorf("invalid string length: %d", strlen)
	}

	b.Pos++

	// Check for enough data
	if b.Pos+uint32(strlen) > uint32(len(b.Data)) {
		return nil, fmt.Errorf("string length %d exceeds buffer at pos %d", strlen, b.Pos)
	}

	// Extract string data
	stringData := make([]byte, strlen)
	copy(stringData, b.Data[b.Pos:b.Pos+uint32(strlen)])
	b.Pos += uint32(strlen)

	// Create value object
	value, err := NewValue(stringData)
	if err != nil {
		return nil, err
	}

	// Create value node
	valueNode, err := NewBValueNode(off, value)
	if err != nil {
		return nil, err
	}

	valueNode.SetLength(b.Pos - off)
	return valueNode, nil
}

func (b *BDecoder) parseInt() (Node, error) {
	off := b.Pos
	if b.Data[b.Pos] != 'i' {
		return nil, fmt.Errorf("expected 'i' at position %d", b.Pos)
	}
	b.Pos++
	start := b.Pos

	if b.Pos < uint32(len(b.Data)) {
		if b.Data[b.Pos] == '-' {
			if b.Pos+1 >= uint32(len(b.Data)) {
				return nil, fmt.Errorf("invalid '-' at end of integer at %d", b.Pos)
			}
			if b.Data[b.Pos+1] == '0' {
				return nil, fmt.Errorf("negative zero not allowed at %d", b.Pos)
			}
		} else if b.Data[b.Pos] == '0' {
			if b.Pos+1 < uint32(len(b.Data)) && b.Data[b.Pos+1] != 'e' {
				return nil, fmt.Errorf("leading zero in integer at %d", b.Pos)
			}
		}
	}

	for b.Pos < uint32(len(b.Data)) && b.Data[b.Pos] != 'e' {
		if (b.Data[b.Pos] < '0' || b.Data[b.Pos] > '9') && b.Data[b.Pos] != '-' {
			return nil, fmt.Errorf("invalid integer byte: %q", b.Data[b.Pos])
		}
		b.Pos++
	}

	if b.Pos >= uint32(len(b.Data)) || b.Data[b.Pos] != 'e' {
		return nil, fmt.Errorf("unterminated integer starting at %d", start)
	}

	intStr := string(b.Data[start:b.Pos])
	intVal, err := strconv.ParseInt(intStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer format: %w", err)
	}
	b.Pos++

	value, err := NewValue(intVal)
	if err != nil {
		return nil, err
	}

	node, err := NewBValueNode(off, value)
	if err != nil {
		return nil, err
	}
	node.SetLength(b.Pos - off)
	return node, nil
}

func (b *BDecoder) parseList() (Node, error) {
	off := b.Pos
	b.Level++

	// Create the list node
	curr, err := NewBListNode(off)
	if err != nil {
		return nil, err
	}
	b.Pos++

	for b.Pos < uint32(len(b.Data)) && b.Data[b.Pos] != 'e' {
		child, err := b.Decode()
		if err != nil {
			return nil, fmt.Errorf("error parsing list element: %w", err)
		}
		curr.AddChild(child)
	}

	if b.Pos >= uint32(len(b.Data)) || b.Data[b.Pos] != 'e' {
		return nil, fmt.Errorf("expected 'e' to end list at pos %d", b.Pos)
	}

	b.Pos++
	b.Level--
	curr.SetLength(b.Pos - off)

	return curr, nil
}

func (b *BDecoder) parseDict() (Node, error) {
	off := b.Pos

	// Create the dictionary node
	curr, err := NewBDictNode(off)
	if err != nil {
		return nil, err
	}

	b.Pos++
	b.Level++

	for b.Pos < uint32(len(b.Data)) && b.Data[b.Pos] != 'e' {
		// Parse key (string!)
		keyNode, err := b.Decode()
		if err != nil {
			return nil, fmt.Errorf("error parsing dict key: %w", err)
		}
		if keyNode.GetType() != VALUE {
			return nil, fmt.Errorf("dict key must be string, got type %d", keyNode.GetType())
		}

		// Extract key data
		/*
			keyData := b.Data[keyNode.GetOffset() : keyNode.GetOffset()+keyNode.GetLength()]
			colonPos := -1
			for i, c := range keyData {
				if c == ':' {
					colonPos = i
					break
				}
			}
			if colonPos == -1 {
				return nil, fmt.Errorf("malformed string key")
			}
			actualKey := keyData[colonPos+1:]
		*/
		vn, ok := AsValueNode(keyNode)
		if !ok {
			return nil, fmt.Errorf("dict key must be ValueNode")
		}
		actualKey := vn.GetValue().Strval

		// Parse value
		entryNode, err := b.Decode()
		if err != nil {
			return nil, fmt.Errorf("error parsing dict value: %w", err)
		}

		// Create dictionary  key
		entry, err := NewBDictEntry(actualKey, entryNode)
		if err != nil {
			return nil, err
		}
		curr.AddEntry(entry)
	}

	if b.Pos >= uint32(len(b.Data)) || b.Data[b.Pos] != 'e' {
		return nil, fmt.Errorf("expected 'e' to end dict at pos %d", b.Pos)
	}

	b.Pos++
	b.Level--
	curr.SetLength(b.Pos - off)

	return curr, nil
}
