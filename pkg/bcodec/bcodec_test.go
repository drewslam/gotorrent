package bcodec_test

import (
	"bytes"
	"testing"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

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

	decb, _ := encodeToBytes(decoded)
	if !bytes.Equal(buf, decb) {
		t.Errorf("mismatch:\noriginal: %q\ndecoded: %q", buf, decb)
	}
}

func TestEncodeDecodeInt(t *testing.T) {
	val, _ := bcodec.NewValue(int64(1644))
	node, _ := bcodec.NewBValueNode(0, val)
	RoundTrip(t, node)
}

func TestNegativeAndZero(t *testing.T) {
	for _, n := range []int64{0, -1, -9999999999} {
		val, _ := bcodec.NewValue(n);
		node, _ := bcodec.NewBValueNode(0, val)
		RoundTrip(t, node)
	}
}

func TestEncodeDecodeString(t *testing.T) {
	val, _ := bcodec.NewValue([]byte("Forza Ferrari"))
	node, _ := bcodec.NewBValueNode(0, val)
	RoundTrip(t, node)
}

func TestEncodeDecodeEmptyString(t *testing.T) {
	val, _ := bcodec.NewValue("")
	node, _ := bcodec.NewBValueNode(0, val)
	RoundTrip(t, node)
}

func TestEncodeDecodeList(t *testing.T) {
	list, _ := bcodec.NewBListNode(0)
	val1, _ := bcodec.NewValue(int64(5))
	val2, _ := bcodec.NewValue([]byte("hella"))
	n1, _ := bcodec.NewBValueNode(0, val1)
	n2, _ := bcodec.NewBValueNode(0, val2)
	list.AddChild(n1)
	list.AddChild(n2)
	RoundTrip(t, list)
}

func TestEncodeDecodeDict(t *testing.T) {
	dict, _ := bcodec.NewBDictNode(0)
	val, _ := bcodec.NewValue([]byte("Smooth"))
	nv, _ := bcodec.NewBValueNode(0, val)
	entry, _ := bcodec.NewBDictEntry([]byte("Operator"), nv)
	dict.AddEntry(entry)
	RoundTrip(t, dict)
}

func TestEncodeDecodeEmptyList(t *testing.T) {
	list, _ := bcodec.NewBListNode(0)
	RoundTrip(t, list)
}

