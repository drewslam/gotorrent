/*
 * title: gotorrent-tracker
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"github.com/drewslam/gotorrent/pkg/bcodec"
)

// Response
type Response struct {
	Reason   string
	Interval uint64
	Peers    []*Peer
	Seeders  uint32
	Leechers uint32
}

func NewResponse(data []*bcodec.BDictEntry) *Response {
	var reason string
	var interval uint64
	var peerList []*Peer
	var seeders uint32
	var leechers uint32

	for _, c := range data {
		if string(c.Key) == "reason" || string(c.Key) == "failure reason" {
			if node, ok := bcodec.AsValueNode(c.Value); ok {
				reason = string(node.GetValue().Strval)
				return &Response{
					Reason: reason,
				}
			}
		} else {
			switch string(c.Key) {
			case "interval":
				interval = extractInterval(c.Value)
			case "peers":
				peerList = extractPeerList(c.Value)
			case "complete":
				seeders = extractUint32(c.Value)
			case "incomplete":
				leechers = extractUint32(c.Value)
			}
		}
	}

	return &Response{
		Interval: interval,
		Peers:    peerList,
		Seeders:  seeders,
		Leechers: leechers,
	}
}
