 /*
 * title: gotorrent-tracker request
 * author: Andrew Souza
 * GPLv3
 */

package tracker

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

