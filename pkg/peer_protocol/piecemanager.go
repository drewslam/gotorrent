/*
 *
 * title: gotorrent peer_protocol piecemanager
 * author: Andrew Souza
 * license: GPLv3
 */
package peer_protocol

import (
	"sync"

	"github.com/drewslam/gotorrent/pkg/torrent"
)

type PieceManager struct {
	NumPieces  uint32
	HavePieces []byte

	InProgress map[uint32]bool

	mu sync.RWMutex
}

func NewPieceManager(tor *torrent.Torrent) *PieceManager {
	numPieces := uint32(len(tor.Info.Pieces))
	bitfieldSize := (numPieces + 7) / 8

	return &PieceManager{
		NumPieces:  numPieces,
		HavePieces: make([]byte, bitfieldSize),
		InProgress: make(map[uint32]bool),
	}
}

func (pm *PieceManager) SelectPiece(bitfield []byte) (uint32, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i := range pm.NumPieces {
		if PeerHasPiece(bitfield, i) && !isBitInBitfield(i, pm.HavePieces) && !pm.InProgress[i] {
			pm.InProgress[i] = true
			return i, true
		}
	}

	return 0, false
}

func (pm *PieceManager) MarkComplete(index uint32) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.SetPiece(index)

	delete(pm.InProgress, index)
}

func (pm *PieceManager) ReleasePiece(index uint32) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.InProgress, index)
}

func (pm *PieceManager) IsComplete() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for i := range pm.NumPieces {
		if !isBitInBitfield(i, pm.HavePieces) {
			return false
		}
	}

	return true
}

func (pm *PieceManager) WeHavePiece(index uint32) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return isBitInBitfield(index, pm.HavePieces)
}

func (pm *PieceManager) SetPiece(index uint32) {
	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	pm.HavePieces[byteIndex] |= (1 << bitIndex)
}
