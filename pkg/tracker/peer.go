/*
 * title: gotorrent-tracker peer
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"crypto/rand"
	"net"
)

// Peer
type Peer struct {
	ID   [20]byte
	IP   net.IP
	Port uint16
}

func NewPeer(t ...any) *Peer {
	var id [20]byte
	var ip net.IP
	port := DefaultPort
	copy(id[:6], []byte(IdPrefix))
	rand.Read(id[6:])

	for _, arg := range t {
		if a, ok := arg.(net.IP); ok {
			ip = a
		}
		if a, ok := arg.(uint16); ok {
			port = a
		}
	}

	return &Peer{
		ID:   id,
		IP:   ip,
		Port: port,
	}
}
