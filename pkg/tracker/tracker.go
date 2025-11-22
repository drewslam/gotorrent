/*
 * title: gotorrent-tracker
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import "strconv"

const defaultPort = 6881
const magicVal = "DSGT01"

type Event int

const (
	STARTED Event = iota
	STOPPED
	COMPLETED
)

type Peer struct {
	ID   []byte
	IP   string
	Port int
}

func NewPeer() *Peer {
	return &Peer{
		ID:   []byte(magicVal + "00000000000001"),
		Port: defaultPort,
	}
}

type Request struct {
	InfoHash   [20]byte
	Peer       *Peer
	Uploaded   uint64
	Downloaded uint64
	Left       uint64
	Event      Event
}

func NewRequest(ih [20]byte, peer *Peer, filesize uint64) *Request {
	return &Request{
		InfoHash: ih,
		Peer:     peer,
		Uploaded: 0,
		Downloaded: 0,
		Left: filesize,
	}
}

type Response struct {
	Reason   string
	Interval uint64
	PeerDict []*Peer
	Peers    [][6]byte
}

func (r *Request) URL(announce string, path string) string {
	port := strconv.Itoa(r.Peer.Port)
	return announce+"?info_hash="+path+"&peer_id="+string(r.Peer.ID)+"&port="+port
}
