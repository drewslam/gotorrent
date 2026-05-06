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
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

func (r *Request) httpAnnounce(announce *url.URL) (*Response, error) {
	ur, err := r.buildURL(announce.String())
	if err != nil {
		return nil, fmt.Errorf("URL failure: %w", err)
	}

	resp, err := http.Get(ur)
	if err != nil {
		return nil, fmt.Errorf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	rs, err := decodeHTTPResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tracker response: %v", err)
	}

	return rs, nil
}

// URL Processer
func (r *Request) buildURL(announce string) (string, error) {
	infoHash := escapeBytes(r.InfoHash[:])
	peerId := escapeBytes(r.Peer.ID[:])
	uploaded := strconv.FormatUint(r.Uploaded, 10)
	left := strconv.FormatUint(r.Left, 10)
	downloaded := strconv.FormatUint(r.Downloaded, 10)
	event, err := r.Event.String()
	if err != nil {
		return "", fmt.Errorf("EventString failure: %w", err)
	}
	port := strconv.FormatUint(uint64(r.Peer.Port), 10)
	return announce + "?info_hash=" + infoHash +
		"&peer_id=" + peerId +
		"&uploaded=" + uploaded +
		"&left=" + left +
		"&downloaded=" + downloaded +
		"&event=" + event +
		"&compact=1" +
		"&port=" + port, nil
}

func decodeHTTPResponse(resp *http.Response) (*Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	db, err := bcodec.NewBDecoder(body, 0)
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
	res := uint64(1800)
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
		peerList = make([]*Peer, 0, np)
		for i := range np {
			off := i * 6
			ip := net.IPv4(nv[off], nv[off+1], nv[off+2], nv[off+3])
			port := binary.BigEndian.Uint16(nv[off+4 : off+6])
			peerList = append(peerList, NewPeer(ip, port))
		}
	}
	return peerList
}

func extractUint32(node bcodec.Node) uint32 {
	if a, ok := bcodec.AsValueNode(node); ok {
		if a.GetValue().Big_ival >= 0 && a.GetValue().Big_ival <= math.MaxUint32 {
			return uint32(a.GetValue().Big_ival)
		}
	}
	return 0
}

func escapeBytes(data []byte) string {
	var res strings.Builder
	for _, d := range data {
		res.WriteString(fmt.Sprintf("%%%02X", d))
	}
	return res.String()
}
