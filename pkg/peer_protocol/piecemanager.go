/*
 *
 * title: gotorrent peer_protocol piecemanager
 * author: Andrew Souza
 * license: GPLv3
 */
package peer_protocol

import (
	"encoding/binary"
	"fmt"

	// "os"
	"sync"

	"github.com/drewslam/gotorrent/pkg/storage"
	"github.com/drewslam/gotorrent/pkg/torrent"
)

type PieceManager struct {
	DataMgr *DataManager
	PcState map[uint32]*PieceState
	Storage *storage.FileStorage

	mu sync.RWMutex
}

func NewPieceManager(tor *torrent.Torrent, fs *storage.FileStorage) *PieceManager {
	numPieces := uint32(len(tor.Info.Pieces))
	pieceLen := uint32(tor.Info.PieceLen)
	pieces := tor.Info.Pieces
	fileSize := tor.FileSize()

	dm := NewDataManager(pieceLen, fileSize, numPieces, pieces)

	return &PieceManager{
		DataMgr: dm,
		PcState: make(map[uint32]*PieceState),
		Storage: fs,
	}
}

func (pm *PieceManager) SelectPiece(bitfield []byte) (uint32, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i := range pm.DataMgr.NumPieces {
		_, ok := pm.PcState[i]
		if PeerHasPiece(bitfield, i) && !isBitInBitfield(i, pm.DataMgr.Completed) && !ok {
			newState := NewPieceState(i, pm.DataMgr.PieceLength)
			pm.PcState[i] = newState
			return i, true
		}
	}

	return 0, false
}

func (pm *PieceManager) FinishPiece(index uint32, bitfield []byte, verified bool) (*Message, error) {
	if !verified {
		if nextPiece, ok := pm.HandleFailure(index, bitfield); ok {
			return pm.PrepareRequest(nextPiece, 0)
		}

		return nil, nil
	}

	pm.MarkComplete(index)

	if pm.IsComplete() {
		fmt.Printf("Download complete!\n")
		return nil, nil
	}

	if nextPiece, ok := pm.SelectPiece(bitfield); ok {
		return pm.PrepareRequest(nextPiece, 0)
	}

	return nil, nil
}

func (pm *PieceManager) HandleFailure(index uint32, bitfield []byte) (uint32, bool) {
	pm.ReleasePiece(index)
	return pm.SelectPiece(bitfield)
}

func (pm *PieceManager) PrepareRequest(index uint32, offset uint32) (*Message, error) {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], offset)
	binary.BigEndian.PutUint32(payload[8:12], MaxBlockSize)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.PcState[index]; !ok {
		//	if !pm.containsIndex(index) {
		return nil, fmt.Errorf("piece state not found for index %d", index)
	}

	/*if !pm.PcState[index].MarkRequested(offset) {
		return nil, nil
	}
	*/

	return NewMessageWP(13, Request, payload), nil
}

func (pm *PieceManager) MarkComplete(index uint32) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.DataMgr.MarkComplete(index)
	delete(pm.PcState, index)
}

func (pm *PieceManager) ReleasePiece(index uint32) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.PcState, index)
}

func (pm *PieceManager) IsComplete() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for i := range pm.DataMgr.NumPieces {
		if !isBitInBitfield(i, pm.DataMgr.Completed) {
			return false
		}
	}

	return true
}

func (pm *PieceManager) WeHavePiece(index uint32) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return isBitInBitfield(index, pm.DataMgr.Completed)
}

func (pm *PieceManager) PieceSize(index uint32) uint32 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if index == pm.DataMgr.NumPieces-1 {
		rem := pm.DataMgr.TotalLength % uint64(pm.DataMgr.PieceLength)
		if rem != 0 {
			return uint32(rem)
		}
	}
	return pm.DataMgr.PieceLength
}

func (pm *PieceManager) HandleUnchoke(bitfield []byte) (uint32, uint32, uint32, bool) {
	pieceIndex, ok := pm.SelectPiece(bitfield)
	if !ok {
		return 0, 0, MaxBlockSize, false
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	/*	if pm.PcState[pieceIndex] == nil {
		return 0, 0, MaxBlockSize, false
	} */

	nextBlock, nextLength, ok := pm.PcState[pieceIndex].NextBlockToRequest()
	if !ok {
		return 0, 0, MaxBlockSize, false
	}

	return pieceIndex, nextBlock, nextLength, true
}

func (pm *PieceManager) HandlePieceMessage(msg *Message) (uint32, bool) {
	pieceIndex := binary.BigEndian.Uint32(msg.Payload[0:4])
	offset := binary.BigEndian.Uint32(msg.Payload[4:8])
	blockData := msg.Payload[8:]

	fmt.Printf("piece received: index=%d offset=%d\n", pieceIndex, offset)

	pm.mu.Lock()
	if _, ok := pm.PcState[pieceIndex]; !ok {
		// if !pm.containsIndex(pieceIndex) {
		pm.PcState[pieceIndex] = NewPieceState(pieceIndex, pm.DataMgr.PieceSize(pieceIndex))
	}
	pm.mu.Unlock()

	pm.DataMgr.StoreBlock(pieceIndex, offset, blockData)

	pm.mu.Lock()
	pm.PcState[pieceIndex].AddBlock(offset, blockData)
	isComplete := pm.PcState[pieceIndex].IsComplete()
	pm.mu.Unlock()

	return pieceIndex, isComplete
}

/*
func (pm *PieceManager) containsIndex(index uint32) bool {
	_, ok := pm.PcState[index]
	return ok
}
*/

func (pm *PieceManager) GetNextBlock(index uint32) (uint32, uint32, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.PcState[index] == nil {
		return 0, 0, false
	}

	return pm.PcState[index].NextBlockToRequest()
}
