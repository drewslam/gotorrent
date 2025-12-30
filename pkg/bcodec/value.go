/*
 *
 *  title: gotorrent bcodec value
 *  author: Andrew Souza
 *  GPLv3
 *
 *  The bencode parser design for this library was influenced by the implementation in libktorrent from the KDE project.
 *  https://github.com/kde/libktorrent
 *
 */
package bcodec

import "fmt"

type VType int

const (
	STRING VType = iota
	INT
	INT64
)

type Value struct {
	Vtype    VType
	Ival     int32
	Strval   []byte
	Big_ival int64
}

func NewValue(T any) (*Value, error) {
	switch v := T.(type) {
	case int32:
		return &Value{
			Vtype:    INT,
			Ival:     v,
			Big_ival: int64(v),
		}, nil
	case int64:
		return &Value{
			Vtype:    INT64,
			Big_ival: v,
		}, nil
	case []byte:
		return &Value{
			Vtype:    STRING,
			Ival:     0,
			Strval:   append([]byte(nil), v...),
			Big_ival: 0,
		}, nil
	case *Value:
		var strCopy []byte
		if v.Strval != nil {
			strCopy = append([]byte(nil), v.Strval...)
		}
		return &Value{
			Vtype:    v.Vtype,
			Ival:     v.Ival,
			Strval:   strCopy,
			Big_ival: v.Big_ival,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type passed to NewValue: %T", T)
	}
}

func (v *Value) CopyFrom(T any) error {
	switch va := T.(type) {
	case *Value:
		v.Vtype = va.Vtype
		v.Ival = va.Ival
		if va.Strval != nil {
			v.Strval = make([]byte, len(va.Strval))
			copy(v.Strval, va.Strval)
		} else {
			v.Strval = nil
		}
		v.Big_ival = va.Big_ival
		return nil
	case int32:
		v.Vtype = INT
		v.Ival = va
		v.Strval = nil
		v.Big_ival = int64(va)
		return nil
	case int64:
		v.Vtype = INT64
		v.Ival = 0
		v.Strval = nil
		v.Big_ival = va
		return nil
	case []byte:
		v.Vtype = STRING
		v.Ival = 0
		v.Strval = make([]byte, len(va))
		copy(v.Strval, va)
		v.Big_ival = 0
		return nil
	default:
		return fmt.Errorf("unsupported type passed to CopyFrom: %T", T)
	}
}
