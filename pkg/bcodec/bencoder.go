/*
 *
 *  title: gotorrent bencode bencoder
 *  author: Andrew Souza
 *  GPLv3
 *
 *  The bencode parser design for this library was influenced by the implementation in libktorrent from the KDE project.
 *  https://invent.kde.org/network/libktorrent
 *
 */
package bcodec

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

type BEncoder struct {
	Out io.Writer
}

type bufferWriter struct {
	data *[]byte
}

func NewBEncoder(t any) (*BEncoder, error) {
	switch va := t.(type) {
	case *os.File:
		return &BEncoder{Out: va}, nil
	case *[]byte:
		return &BEncoder{Out: &bufferWriter{data: va}}, nil
	case io.Writer:
		return &BEncoder{Out: va}, nil
	default:
		return nil, fmt.Errorf("unsupported type passed to NewBEncoder: %T", t)
	}
}

func (b *BEncoder) Write(t any) error {
	switch va := t.(type) {
	case bool:
		return b.writeBool(va)
	case int:
		return b.writeInt(int64(va))
	case int32:
		return b.writeInt(int64(va))
	case int64:
		return b.writeInt(va)
	case string:
		return b.writeString(va)
	case []byte:
		return b.writeBytes(va)
	case Node:
		return b.writeNode(va)
	case float32, float64:
		return fmt.Errorf("float32 not supported in bencode")
	case uint, uint32, uint64:
		return fmt.Errorf("unsigned ints are not valid bencode")
	default:
		return fmt.Errorf("unsupported type: %T", va)
	}
}

func (b *bufferWriter) Write(p []byte) (int, error) {
	*b.data = append(*b.data, p...)
	return len(p), nil
}

func (b *BEncoder) BeginDict() error {
	_, err := b.Out.Write([]byte("d"))
	return err
}

func (b *BEncoder) BeginList() error {
	_, err := b.Out.Write([]byte("l"))
	return err
}

func (b *BEncoder) End() error {
	_, err := b.Out.Write([]byte("e"))
	return err
}

func (b *BEncoder) writeBool(val bool) error {
	if val {
		_, err := b.Out.Write([]byte("i1e"))
		return err
	}
	_, err := b.Out.Write([]byte("i0e"))
	return err
}

func (b *BEncoder) writeInt(val int64) error {
	data := []byte("i" + strconv.FormatInt(val, 10) + "e")
	_, err := b.Out.Write(data)
	return err
}

func (b *BEncoder) writeString(val string) error {
	length := strconv.Itoa(len(val))
	if _, err := b.Out.Write([]byte(length + ":")); err != nil {
		return err
	}
	_, err := b.Out.Write([]byte(val))
	return err
}

func (b *BEncoder) writeBytes(val []byte) error {
	if val == nil {
		return fmt.Errorf("cannot encode nil []byte")
	}
	length := strconv.Itoa(len(val))
	if _, err := b.Out.Write([]byte(length + ":")); err != nil {
		return err
	}
	_, err := b.Out.Write(val)
	return err
}

func (b *BEncoder) writeValue(val *Value) error {
	if val == nil {
		return fmt.Errorf("cannot encode nil value")
	}
	switch val.Vtype {
	case STRING:
		return b.writeBytes(val.Strval)
	case INT:
		return b.writeInt(int64(val.Ival))
	case INT64:
		return b.writeInt(val.Big_ival)
	default:
		return fmt.Errorf("unknown value type: %d", val.Vtype)
	}
}

func (b *BEncoder) writeNode(node Node) error {
	if node == nil {
		return fmt.Errorf("cannot encode nil node")
	}
	switch node.GetType() {
	case VALUE:
		if vn, ok := AsValueNode(node); ok {
			return b.writeValue(vn.GetValue())
		}
		return fmt.Errorf("node claims to be VALUE but doesn't implement ValueNode")
	case DICT:
		if dn, ok := AsDictNode(node); ok {
			return b.writeDictNode(dn)
		}
		return fmt.Errorf("node claims to be DICT but doesn't implement DictNode")
	case LIST:
		if ln, ok := AsListNode(node); ok {
			return b.writeListNode(ln)
		}
		return fmt.Errorf("node claims to be LIST but doesn't implement ListNode")
	default:
		return fmt.Errorf("unknown node type: %d", node.GetType())
	}
}

func (b *BEncoder) writeDictNode(node DictNode) error {
	if err := b.BeginDict(); err != nil {
		return err
	}

	for _, entry := range node.GetEntries() {
		// write key (always a byte slice)
		if err := b.writeBytes(entry.Key); err != nil {
			return fmt.Errorf("error writing dict key: %w", err)
		}

		// write value
		if err := b.writeNode(entry.Value); err != nil {
			return fmt.Errorf("error writing dict value: %w", err)
		}
	}

	return b.End()
}

func (b *BEncoder) writeListNode(node ListNode) error {
	if err := b.BeginList(); err != nil {
		return err
	}

	for _, child := range node.GetChildren() {
		if err := b.writeNode(child); err != nil {
			return fmt.Errorf("error writing list element: %w", err)
		}
	}

	return b.End()
}

// Convenience method for encoding complete structures
func (b *BEncoder) EncodeDict(entries map[string]any) error {
	if err := b.BeginDict(); err != nil {
		return err
	}

	for key, value := range entries {
		if err := b.writeString(key); err != nil {
			return fmt.Errorf("error writing dict value for key '%s': %w", key, err)
		}
		if err := b.Write(value); err != nil {
			return fmt.Errorf("error writing dict value for key '%s': %w", key, err)
		}
	}

	return b.End()
}

func (b *BEncoder) EncodeList(items []any) error {
	if err := b.BeginList(); err != nil {
		return err
	}

	for i, item := range items {
		if err := b.Write(item); err != nil {
			return fmt.Errorf("error writing list item at index %d: %w", i, err)
		}
	}

	return b.End()
}
