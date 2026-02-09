/*
 *
 * title: gotorrent peer_protocol datamanager
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import (
	"crypto/sha1"
	"sync"
)

type DataManager struct {
	PieceLength uint32
	TotalLength uint64
	NumPieces   uint32
	PiecesHash  [][20]byte

	Data      []byte
	Completed []byte

	mu sync.RWMutex
}

func NewDataManager(pl uint32, tl uint64, np uint32, ph [][20]byte) *DataManager {
	bitfieldSize := (np + 7) / 8
	return &DataManager{
		PieceLength: pl,
		TotalLength: tl,
		NumPieces:   np,
		PiecesHash:  ph,
		Data:        make([]byte, tl),
		Completed:   make([]byte, bitfieldSize),
	}
}

func (dm *DataManager) VerifyPiece(pieceIndex uint32, data []byte) bool {
	sum := sha1.Sum(data)
	return sum == dm.PiecesHash[pieceIndex]
}

func (dm *DataManager) StoreBlock(index uint32, offset uint32, data []byte) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pieceStart := uint64(index) * uint64(dm.PieceLength)
	blockStart := pieceStart + uint64(offset)

	copy(dm.Data[blockStart:], data)
}

func (dm *DataManager) AssemblePiece(pieceIndex uint32) []byte {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pieceSize := dm.PieceSize(pieceIndex)
	start := uint64(pieceIndex) * uint64(dm.PieceLength)
	end := start + uint64(pieceSize)

	piece := make([]byte, pieceSize)
	copy(piece, dm.Data[start:end])

	return piece
}

func (dm *DataManager) PieceSize(index uint32) uint32 {
	if index == dm.NumPieces - 1 {
		rem := dm.TotalLength % uint64(dm.PieceLength)
		if rem != 0 {
			return uint32(rem)
		}
	}

	return dm.PieceLength
}

func (dm *DataManager) MarkComplete(index uint32) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	dm.Completed[byteIndex] |= (1 << bitIndex)
}

func (dm *DataManager) IsComplete(index uint32) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	return dm.Completed[byteIndex] & (1 << bitIndex) != 0
}
