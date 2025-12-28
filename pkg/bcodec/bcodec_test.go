package bcodec_test

import (
	"bytes"
	"testing"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

func TestEncodeDecodeInt(t *testing.T) {
	val, err := bcodec.NewValue(int64(1644))
	if err != nil {
		t.Fatalf("failed to create value: %v", err)
	}
	node, err := bcodec.NewBValueNode(0, val)
	if err != nil {
		t.Fatalf("failed to create value node: %v", err)
	}
	RoundTrip(t, node)
}

func TestEncodeDecodeString(t *testing.T) {
	val, err := bcodec.NewValue([]byte("Forza Ferrari"))
	if err != nil {
		t.Fatalf("failed to create value: %v", err)
	}
	node, err := bcodec.NewBValueNode(0, val)
	if err != nil {
		t.Fatalf("failed to create value node: %v", err)
	}
	RoundTrip(t, node)
}

func TestEncodeDecodeList(t *testing.T) {
	list, err := bcodec.NewBListNode(0)
	if err != nil {
		t.Fatalf("failed to create list node: %v", err)
	}
	val1, err := bcodec.NewValue(int64(5))
	if err != nil {
		t.Fatalf("failed to create value: %v", err)
	}
	val2, err := bcodec.NewValue([]byte("hella"))
	if err != nil {
		t.Fatalf("failed to create value: %v", err)
	}
	n1, err := bcodec.NewBValueNode(0, val1)
	if err != nil {
		t.Fatalf("failed to create value node: %v", err)
	}
	n2, err := bcodec.NewBValueNode(0, val2)
	if err != nil {
		t.Fatalf("failed to create value node: %v", err)
	}
	list.AddChild(n1)
	list.AddChild(n2)
	RoundTrip(t, list)
}

func TestEncodeDecodeDict(t *testing.T) {
	dict, err := bcodec.NewBDictNode(0)
	if err != nil {
		t.Fatalf("failed to create dictionary node: %v", err)
	}
	val, err := bcodec.NewValue([]byte("Smooth"))
	if err != nil {
		t.Fatalf("failed to create value: %v", err)
	}
	nv, err := bcodec.NewBValueNode(0, val)
	if err != nil {
		t.Fatalf("failed to create value node: %v", err)
	}
	entry, err := bcodec.NewBDictEntry([]byte("Operator"), nv)
	if err != nil {
		t.Fatalf("failed to create dictionary entry: %v", err)
	}
	dict.AddEntry(entry)
	RoundTrip(t, dict)
}

func TestNegativeAndZero(t *testing.T) {
	for _, n := range []int64{0, -1, -9999999999} {
		val, err := bcodec.NewValue(int64(n))
		if err != nil {
			t.Fatalf("failed to create value: %v", err)
		}
		node, err := bcodec.NewBValueNode(0, val)
		if err != nil {
			t.Fatalf("failed to create value node: %v", err)
		}
		RoundTrip(t, node)
	}
}

func TestEncodeDecodeEmptyString(t *testing.T) {
	byt, err := encodeToBytes("")
	if err != nil {
		t.Fatalf("failed to encode empty string: %v", err)
	}
	val, err := bcodec.NewValue(byt)
	if err != nil {
		t.Fatalf("failed to create value: %v", err)
	}
	node, err := bcodec.NewBValueNode(0, val)
	if err != nil {
		t.Fatalf("failed to create value node: %v", err)
	}
	RoundTrip(t, node)
}

func TestEncodeDecodeEmptyList(t *testing.T) {
	list, err := bcodec.NewBListNode(0)
	if err != nil {
		t.Fatalf("failed to create list node: %v", err)
	}
	RoundTrip(t, list)
}

func RoundTrip(t *testing.T, elem any) {
	t.Helper()

	var buf []byte
	enc, err := bcodec.NewBEncoder(&buf)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}
	if err := enc.Write(elem); err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dec, err := bcodec.NewBDecoder(buf, false, 0)
	if err != nil {
		t.Fatalf("failed to create decoder: %v", err)
	}
	decoded, err := dec.Decode()
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	decb, err := encodeToBytes(decoded)
	if err != nil {
		t.Fatalf("encoder error: %v", err)
	}
	if !bytes.Equal(buf, decb) {
		t.Errorf("mismatch:\noriginal: %q\ndecoded: %q", buf, decb)
	}
}

func encodeToBytes(v any) ([]byte, error) {
	var buf []byte
	enc, err := bcodec.NewBEncoder(&buf)
	if err != nil {
		return nil, err
	}
	if err := enc.Write(v); err != nil {
		return nil, err
	}
	return buf, nil
}
