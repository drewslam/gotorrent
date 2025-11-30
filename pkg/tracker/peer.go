/*
 * title: gotorrent-tracker peer
 * author: Andrew Souza
 * GPLv3
 */

package tracker

// Peer
type Peer struct {
	ID   []byte
	IP   string
	Port int
}

func NewPeer(t ...any) *Peer {
	peer := &Peer{
		ID:   []byte(IdPrefix + "00000000000001"),
		Port: DefaultPort,
	}

	if t == nil {
		return peer
	}

	for _, i := range t {
		if a, ok := i.([]byte); ok {
			peer.ID = a
		}
		if a, ok := i.(string); ok {
			peer.IP = a
		}
		if a, ok := i.(int); ok {
			peer.Port = a
		}
	}

	return peer
}

