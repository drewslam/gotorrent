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
)

const defaultPort = 6881
const idPrefix = "DSGT01"

type Peer struct {
	ID   []byte
	IP   string
	Port int
}

func NewPeer() *Peer {
	return &Peer{
		ID:   []byte(idPrefix + "00000000000001"),
		Port: defaultPort,
	}
}

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

type Response struct {
	Reason   string
	Interval uint64
	PeerDict []*Peer
	Peers    [][6]byte
}

func escapeString(data []byte) string {
	var res strings.Builder
	for _, b := range data {
		res.WriteString(fmt.Sprintf("%%%02X", b))
	}
	return res.String()
}

func (r *Request) URL(announce string) string {
	port := strconv.Itoa(r.Peer.Port)
	uploaded := strconv.FormatUint(r.Uploaded, 10)
	downloaded := strconv.FormatUint(r.Downloaded, 10)
	left := strconv.FormatUint(r.Left, 10)
	peerId := escapeString(r.Peer.ID)
	infoHash := escapeString(r.InfoHash[:])
	return announce + "?info_hash=" + infoHash +
		"&peer_id=" + peerId +
		"&uploaded=" + uploaded +
		"&downloaded=" + downloaded +
		"&left=" + left +
		"&event=" + r.Event +
		"&compact=1" +
		"&port=" + port
}
