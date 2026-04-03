/*
 *
 * title: gotorrent connection_manager peerregistry
 * author: Andrew Souza
 * license: GPLv3
 *
 */

package connection_manager

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/drewslam/gotorrent/pkg/peer_protocol"
	"github.com/drewslam/gotorrent/pkg/tracker"
)

const MaxPeerConnections uint32 = 60

type indexedPeer struct {
	index uint32
	peer  *tracker.Peer
}

type PeerStat struct {
	Tier            int
	LastSuccess     time.Time
	StrikeCount     int
	RestrictedUntil time.Time
}

func NewPeerStat() *PeerStat {
	return &PeerStat{
		Tier:        3,
		StrikeCount: 0,
	}
}

type PeerRegistry struct {
	peerList              []*tracker.Peer
	ActivePeerConnections map[uint32]*peer_protocol.PeerConn
	History               map[string]*PeerStat
	jobQueue              chan indexedPeer
	mu                    sync.RWMutex
}

func NewPeerRegistry(numWorkers int, req *tracker.Request, pm *peer_protocol.PieceManager) *PeerRegistry {
	pr := &PeerRegistry{
		ActivePeerConnections: make(map[uint32]*peer_protocol.PeerConn, MaxPeerConnections),
		History:               make(map[string]*PeerStat, MaxPeerConnections),
		jobQueue:              make(chan indexedPeer, MaxPeerConnections),
	}

	for range numWorkers {
		go pr.peerWorker(req, pm)
	}

	return pr

}

func (pr *PeerRegistry) Register(index uint32, conn *peer_protocol.PeerConn) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if _, ok := pr.ActivePeerConnections[index]; !ok {
		pr.ActivePeerConnections[index] = conn
	}
}

func (pr *PeerRegistry) Deregister(index uint32) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	delete(pr.ActivePeerConnections, index)
}

func (pr *PeerRegistry) IsAvailable(index uint32, addr string) bool {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.isAvailable(index, addr)
}

func (pr *PeerRegistry) isAvailable(index uint32, addr string) bool {
	peer, exists := pr.History[addr]
	if !exists {
		return false
	}
	expiry := peer.RestrictedUntil
	if time.Now().Before(expiry) {
		return false
	}
	_, active := pr.ActivePeerConnections[index]
	return !active
}

func (pr *PeerRegistry) RecordHandshake(addr string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if _, ok := pr.History[addr]; !ok {
		return
	}
	if pr.History[addr].Tier != 1 {
		pr.History[addr].Tier = 2
	}
}

func (pr *PeerRegistry) RecordSuccess(addr string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if _, ok := pr.History[addr]; !ok {
		return
	}
	pr.History[addr].LastSuccess = time.Now()
	pr.History[addr].Tier = 1
	pr.History[addr].StrikeCount = 0
	pr.History[addr].RestrictedUntil = time.Time{}
}

func (pr *PeerRegistry) RecordFailure(addr string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if _, ok := pr.History[addr]; !ok {
		return
	}

	pr.History[addr].StrikeCount++
	strikes := pr.History[addr].StrikeCount
	if pr.History[addr].Tier == 0 {
		pr.History[addr].Tier = 3
	}

	banDuration := time.Duration(strikes) * 30 * time.Second
	if banDuration > 10*time.Minute {
		banDuration = 10 * time.Minute
	}

	pr.History[addr].RestrictedUntil = time.Now().Add(banDuration)
}

func (pr *PeerRegistry) SelectPeers(peerList []*tracker.Peer, want uint32) []indexedPeer {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	if want == 0 {
		return nil
	}

	var tiers [3][]indexedPeer
	for i, peer := range peerList {
		if r, ok := pr.History[peer.Address()]; ok {
			p := indexedPeer{index: uint32(i), peer: peer}
			tier := r.Tier
			switch tier {
			case 1:
				tiers[0] = append(tiers[0], p)
			case 2:
				tiers[1] = append(tiers[1], p)
			default:
				tiers[2] = append(tiers[2], p)
			}
		}
	}

	for i := range tiers {
		rand.Shuffle(len(tiers[i]), func(a, b int) {
			tiers[i][a], tiers[i][b] = tiers[i][b], tiers[i][a]
		})
	}

	var res []indexedPeer
	for _, tier := range tiers {
		for _, peer := range tier {
			if uint32(len(res)) >= want {
				return res
			}

			if ok := pr.isAvailable(peer.index, peer.peer.Address()); ok {
				res = append(res, peer)
			}
		}
	}

	return res
}

func (pr *PeerRegistry) peerWorker(req *tracker.Request, pm *peer_protocol.PieceManager) {
	for p := range pr.jobQueue {
		addr := p.peer.Address()
		err := peer_protocol.HandleConnection(
			addr,
			p.index,
			req,
			pm,
			func(conn *peer_protocol.PeerConn) {
				pr.RecordHandshake(addr)
				pr.Register(p.index, conn)
				pr.RecordSuccess(addr)
			},
			func(pieceIndex uint32) {
				pr.RecordSuccess(addr)
				pr.BroadcastHave(pieceIndex)
			},
		)
		if err != nil {
			pr.RecordFailure(addr)
		}
		pr.Deregister(p.index)
	}
}

func (pr *PeerRegistry) ManagePeerConnections(want uint32) {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		for _, peer := range pr.peerList {
			addr := peer.Address()
			pr.mu.Lock()
			if _, ok := pr.History[addr]; !ok {
				ps := NewPeerStat()
				pr.History[addr] = ps
			}
			pr.mu.Unlock()
		}

		peers := pr.SelectPeers(pr.peerList, want)
		for _, peer := range peers {
			pr.jobQueue <- peer
		}
	}
}

func (pr *PeerRegistry) UpdatePeerList(peers []*tracker.Peer) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.peerList = peers

	for _, peer := range peers {
		if _, ok := pr.History[peer.Address()]; ok {
			pr.History[peer.Address()] = NewPeerStat()
		}
	}
}

func (pr *PeerRegistry) ClearExpiredBans() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		pr.mu.Lock()
		now := time.Now()

		for addr, stat := range pr.History {
			if stat.StrikeCount > 0 && now.After(stat.RestrictedUntil) {
				fmt.Printf("ban expired for %s. resetting strikes,\n", addr)
				stat.StrikeCount = 0
			}

			if stat.StrikeCount == 0 && now.Sub(stat.LastSuccess) > 24*time.Hour {
				if !pr.isPeerActive(addr) {
					delete(pr.History, addr)
				}
			}
		}
		pr.mu.Unlock()
	}
}

func (pr *PeerRegistry) isPeerActive(addr string) bool {
	for _, conn := range pr.ActivePeerConnections {
		if conn.Conn.RemoteAddr().String() == addr {
			return true
		}
	}
	return false
}

func (pr *PeerRegistry) BroadcastHave(pieceIndex uint32) {
	pr.mu.RLock()
	conns := make([]*peer_protocol.PeerConn, 0, len(pr.ActivePeerConnections))
	for _, conn := range pr.ActivePeerConnections {
		conns = append(conns, conn)
	}
	pr.mu.RUnlock()

	if len(conns) == 0 {
		return
	}

	havePayload := make([]byte, 4)
	binary.BigEndian.PutUint32(havePayload, pieceIndex)
	haveMsg := peer_protocol.NewMessageWP(5, peer_protocol.Have, havePayload)
	serialized := haveMsg.Serialize()

	for _, conn := range conns {
		go func(c *peer_protocol.PeerConn) {
			if c.Conn == nil {
				return
			}

			c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			c.Conn.Write(serialized)
		}(conn)
	}
	fmt.Printf("*** broadcast have(%d) to %d peers\n", pieceIndex, len(conns))
}

func RunAnnounceLoop(req *tracker.Request) {

}
