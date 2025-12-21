/*
 * title: gotorrent-tracker http
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

// URL Processer
func (r *Request) URL(announce string) string {
	port := strconv.FormatUint(uint64(r.Peer.Port), 10)
	uploaded := strconv.FormatUint(r.Uploaded, 10)
	downloaded := strconv.FormatUint(r.Downloaded, 10)
	left := strconv.FormatUint(r.Left, 10)
	peerId := EscapeBytes(r.Peer.ID[:])
	infoHash := EscapeBytes(r.InfoHash[:])
	return announce + "?info_hash=" + infoHash +
		"&peer_id=" + peerId +
		"&uploaded=" + uploaded +
		"&downloaded=" + downloaded +
		"&left=" + left +
		"&event=" + r.Event +
		"&compact=1" +
		"&port=" + port
}

func (r *Request) FetchHttpResponse(announce string) (*Response, error) {
	ur := r.URL(announce)

	resp, err := http.Get(ur)
	if err != nil {
		return nil, fmt.Errorf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	rs, err := DecodeHTTPResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tracker response: %v", err)
	}

	return rs, nil
}

func DecodeHTTPResponse(resp *http.Response) (*Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	db, err := bcodec.NewBDecoder(body, false, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bdecoder: %v", err)
	}

	dbc, err := db.DecodeDict()
	if err != nil {
		return nil, fmt.Errorf("failed to decoder response: %v", err)
	}

	return NewResponse(dbc.Children), nil
}

func extractInterval(node bcodec.Node) uint64 {
	res := uint64(0)
	if a, ok := bcodec.AsValueNode(node); ok {
		res = uint64(a.GetValue().Big_ival)
	}
	return res
}

func extractPeerList(node bcodec.Node) []*Peer {
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
		for _, n := range nodeList {
			host := net.IP(n[0:4])
			port := binary.BigEndian.Uint16(n[4:6])
			peer := NewPeer(host, port)
			peerList = append(peerList, peer)
		}
	}
	return peerList
}
