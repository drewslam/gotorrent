/*
 * title: gotorrent-tracker
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"encoding/binary"
	"net"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

// Response
type Response struct {
	Reason   string
	Interval uint64
	Peers    [][6]byte
	PeerDict []*Peer
}

func extractInterval(node bcodec.Node) uint64 {
	res := uint64(0)
	if a, ok := bcodec.AsValueNode(node); ok {
		res = uint64(a.GetValue().Big_ival)
	}
	return res
}

func extractPeerList(node bcodec.Node) ([][6]byte, []*Peer) {
	var peers [][6]byte
	var peerList []*Peer
	if a, ok := bcodec.AsValueNode(node); ok {
		nv := a.GetValue().Strval
		np := len(nv) / 6
		var nodeList [][6]byte
		for i := range np {
			off := i * 6
			var temp [6]byte
			copy(temp[:], nv[off:off+6])
			nodeList = append(nodeList, temp)
		}
		peers = nodeList
		for _, n := range nodeList {
			host := net.IP(n[0:4]).String()
			port := int(binary.BigEndian.Uint16(n[4:6]))
			peer := NewPeer(host, port)
			peerList = append(peerList, peer)
		}
	}
	return peers, peerList
}

func NewResponse(data []*bcodec.BDictEntry) *Response {
	resp := &Response{}

	for _, c := range data {
		if string(c.Key) == "reason" || string(c.Key) == "failure reason" {
			if node, ok := bcodec.AsValueNode(c.Value); ok {
				resp.Reason = string(node.GetValue().Strval)
				break
			}
		} else {
			switch string(c.Key) {
			case "interval":
				interval := extractInterval(c.Value)
				resp.Interval = interval
			case "peers":
				peers, peerList := extractPeerList(c.Value)
				resp.Peers = peers
				resp.PeerDict = peerList
			}
		}
	}

	return resp
}


