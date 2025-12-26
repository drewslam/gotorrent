/*
 * title: gotorrent-tracker request
 * author: Andrew Souza
 * GPLv3
 */

package tracker

type Request struct {
	InfoHash   [20]byte
	Peer       *Peer
	Downloaded uint64
	Left       uint64
	Uploaded   uint64
	Event      Event
	Key uint32
}

func NewRequest(ih [20]byte, peer *Peer, filesize uint64) *Request {
	return &Request{
		InfoHash:   ih,
		Peer:       peer,
		Downloaded: 0,
		Left:       filesize,
		Uploaded:   0,
		Event:      EventStarted,
		Key: NewUint32(),
	}
}

// func (r *Request) Announuce(url string) (*Response, error) {}
