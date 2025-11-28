/*
 * title: gotorrent-tracker
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"fmt"
	"strconv"
	"strings"
	"net"
	"encoding/binary"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

const defaultPort = 6881
const idPrefix = "DSGT01"

func escapeBytes(data []byte) string {
	var res strings.Builder
	for _, d := range data {
		res.WriteString(fmt.Sprintf("%%%02X", d))
	}
	return res.String()
}

// Peer
type Peer struct {
	ID   []byte
	IP   string
	Port int
}

func NewPeer(t ...any) *Peer {
	peer := &Peer{
		ID:   []byte(idPrefix + "00000000000001"),
		Port: defaultPort,
	}

	if t == nil {
		return peer
	}

	for _, i := range t {
		if a, ok := i.([]byte); ok {
			peer.ID = a
		}
		if a, ok := i.(string); ok {
			peer.IP = a
		}
		if a, ok := i.(int); ok {
			peer.Port = a
		}
	}

	return peer
}

// GET Request
type Request struct {
	InfoHash   [20]byte
	Peer       *Peer
	Uploaded   uint64
	Downloaded uint64
	Left       uint64
	Event      string
}

func NewRequest(ih [20]byte, peer *Peer, filesize uint64) *Request {
	return &Request{
		InfoHash:   ih,
		Peer:       peer,
		Uploaded:   0,
		Downloaded: 0,
		Left:       filesize,
		Event:      "started",
	}
}

// Response
type Response struct {
	Reason   string
	Interval uint64
	Peers    [][6]byte
	PeerDict []*Peer
}

func NewResponse(data []*bcodec.BDictEntry) *Response {
	resp := &Response{}

	for _, c := range data {
		if string(c.Key) == "reason" {
			if node, ok := bcodec.AsValueNode(c.Value); ok {
				resp.Reason = string(node.GetValue().Strval)
				break
			}
		} else {
			if string(c.Key) == "interval" {
				if node, ok := bcodec.AsValueNode(c.Value); ok {
					resp.Interval = uint64(node.GetValue().Big_ival)
				}
			}
			if string(c.Key) == "peers" {
				if node, ok := bcodec.AsValueNode(c.Value); ok {
					nv := node.GetValue().Strval
					np := len(nv) / 6
					var nodeList [][6]byte
					for i := range np {
						off := i * 6
						var temp [6]byte
						copy(temp[:], nv[off:off+6])
						nodeList = append(nodeList, temp)
					}
					resp.Peers = nodeList
					for _, n := range nodeList {
						host := net.IP(n[0:4]).String()
						port := int(binary.BigEndian.Uint16(n[4:6]))
						peer := NewPeer(host, port)
						resp.PeerDict = append(resp.PeerDict, peer)
					}
				}
			}
		}
	}

	return resp
}

// URL Processer
func (r *Request) URL(announce string) string {
	port := strconv.Itoa(r.Peer.Port)
	uploaded := strconv.FormatUint(r.Uploaded, 10)
	downloaded := strconv.FormatUint(r.Downloaded, 10)
	left := strconv.FormatUint(r.Left, 10)
	peerId := escapeBytes(r.Peer.ID)
	infoHash := escapeBytes(r.InfoHash[:])
	return announce + "?info_hash=" + infoHash +
		"&peer_id=" + peerId +
		"&uploaded=" + uploaded +
		"&downloaded=" + downloaded +
		"&left=" + left +
		"&event=" + r.Event +
		"&compact=1" +
		"&port=" + port
}
