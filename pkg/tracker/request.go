/*
 * title: gotorrent-tracker request
 * author: Andrew Souza
 * GPLv3
 */
package tracker

import (
	"fmt"
	"net/url"
)

type Request struct {
	InfoHash   [20]byte
	Peer       *Peer
	Downloaded uint64
	Left       uint64
	Uploaded   uint64
	Event      Event
	Key        uint32
}

func NewRequest(ih [20]byte, peer *Peer, filesize uint64) *Request {
	return &Request{
		InfoHash:   ih,
		Peer:       peer,
		Downloaded: 0,
		Left:       filesize,
		Uploaded:   0,
		Event:      EventStarted,
		Key:        NewUint32(),
	}
}

func (r *Request) Announce(u *url.URL) (*Response, error) {
	switch u.Scheme {
	case "http", "https":
		return r.httpAnnounce(u)
	case "udp":
		return r.udpAnnounce(u)
	case "":
		return nil, fmt.Errorf("no announce scheme detected")
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
}
