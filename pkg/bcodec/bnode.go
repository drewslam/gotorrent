/*
 *
 *  title: bencode-go
 *  author: Andrew Souza
 *  GPLv3
 *
 */
package bcodec

import "fmt"

type NType int

const (
	VALUE NType = iota
	DICT
	LIST
	maxNType = LIST
)

// Parent node interface
type Node interface {
	GetType() NType
	GetOffset() uint32
	GetLength() uint32
	SetLength(length uint32)
}

// Basic node struct embedded in all node types
type BNode struct {
	Type NType
	Off  uint32
	Len  uint32
}

func (n *BNode) GetType() NType {
	return n.Type
}

func (n *BNode) GetOffset() uint32 {
	return n.Off
}

func (n *BNode) GetLength() uint32 {
	return n.Len
}

func (n *BNode) SetLength(length uint32) {
	n.Len = length
}

func NewBNode(tipe NType, off uint32) (*BNode, error) {
	if tipe < VALUE || tipe > maxNType {
		return nil, fmt.Errorf("invalid type passed to NewBNode: %d", tipe)
	}
	return &BNode{
		Type: tipe,
		Off:  off,
		Len:  0,
	}, nil
}

// ValueNode interface
type ValueNode interface {
	Node
	GetValue() *Value
	SetValue(val *Value)
}

// DictNode interface
type DictNode interface {
	Node
	GetEntries() []*BDictEntry
	AddEntry(entry *BDictEntry)
	FindEntry(key []byte) *BDictEntry
}

// ListNode interface
type ListNode interface {
	Node
	GetChildren() []Node
	AddChild(child Node)
	GetChildAt(index int) Node
}

// Concrete implementations

// BValueNode implements ValueNode
type BValueNode struct {
	BNode
	Val *Value
}

func NewBValueNode(off uint32, va *Value) (*BValueNode, error) {
	return &BValueNode{
		BNode: BNode{
			Type: VALUE,
			Off:  off,
			Len:  0,
		},
		Val: va,
	}, nil
}

func (n *BValueNode) GetValue() *Value {
	return n.Val
}

func (n *BValueNode) SetValue(val *Value) {
	n.Val = val
}

// BDictEntry represents a key-value pair in a dictionary
type BDictEntry struct {
	Key   []byte
	Value Node
}

func NewBDictEntry(key []byte, value Node) (*BDictEntry, error) {
	if value == nil {
		return nil, fmt.Errorf("no base node passed to NewBDictEntry")
	}

	newKey := make([]byte, len(key))
	copy(newKey, key)

	return &BDictEntry{
		Key:   newKey,
		Value: value,
	}, nil
}

// BDictNode implements DictNode
type BDictNode struct {
	BNode
	Children []*BDictEntry
}

func NewBDictNode(off uint32) (*BDictNode, error) {
	return &BDictNode{
		BNode: BNode{
			Type: DICT,
			Off:  off,
			Len:  0,
		},
		Children: make([]*BDictEntry, 0),
	}, nil
}

func (n *BDictNode) GetEntries() []*BDictEntry {
	return n.Children
}

func (n *BDictNode) AddEntry(entry *BDictEntry) {
	n.Children = append(n.Children, entry)
}

func (n *BDictNode) FindEntry(key []byte) *BDictEntry {
	for _, entry := range n.Children {
		if bytesEqual(entry.Key, key) {
			return entry
		}
	}
	return nil
}

// BListNode implements ListNode
type BListNode struct {
	BNode
	Children []Node
}

func NewBListNode(off uint32) (*BListNode, error) {
	return &BListNode{
		BNode: BNode{
			Type: LIST,
			Off:  off,
			Len:  0,
		},
		Children: make([]Node, 0),
	}, nil
}

func (n *BListNode) GetChildren() []Node {
	return n.Children
}

func (n *BListNode) AddChild(child Node) {
	n.Children = append(n.Children, child)
}

func (n *BListNode) GetChildAt(index int) Node {
	if index < 0 || index >= len(n.Children) {
		return nil
	}
	return n.Children[index]
}

// Helper functions
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Type assertion helpers for safe casting
func AsValueNode(node Node) (ValueNode, bool) {
	if node.GetType() != VALUE {
		return nil, false
	}
	vn, ok := node.(ValueNode)
	return vn, ok
}

func AsDictNode(node Node) (DictNode, bool) {
	if node.GetType() != DICT {
		return nil, false
	}
	dn, ok := node.(DictNode)
	return dn, ok
}

func AsListNode(node Node) (ListNode, bool) {
	if node.GetType() != LIST {
		return nil, false
	}
	ln, ok := node.(ListNode)
	return ln, ok
}

// Visitor pattern for traversing nodes
type NodeVisitor interface {
	VisitValue(node ValueNode) error
	VisitDict(node DictNode) error
	VisitList(node ListNode) error
}

func VisitNode(node Node, visitor NodeVisitor) error {
	switch node.GetType() {
	case VALUE:
		if vn, ok := AsValueNode(node); ok {
			return visitor.VisitValue(vn)
		}
		return fmt.Errorf("node claims to be VALUE but doesn't implement ValueNode")
	case DICT:
		if dn, ok := AsDictNode(node); ok {
			return visitor.VisitDict(dn)
		}
		return fmt.Errorf("node claims to be DICT but doesn't implement DictNode")
	case LIST:
		if ln, ok := AsListNode(node); ok {
			return visitor.VisitList(ln)
		}
		return fmt.Errorf("node claims to be LIST but doesn't implement ListNode")
	default:
		return fmt.Errorf("unknown node type: %d", node.GetType())
	}
}
