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
	"time"

	// "os"
	"sync"

	"github.com/drewslam/gotorrent/pkg/storage"
	"github.com/drewslam/gotorrent/pkg/torrent"
)

type PieceManager struct {
	DataMgr *DataManager
	PcState map[uint32]*PieceState
	Storage *storage.FileStorage

	FailedPieces map[uint32]int
	BannedUntil  map[uint32]time.Time

	mu sync.RWMutex
}

func NewPieceManager(tor *torrent.Torrent, fs *storage.FileStorage) *PieceManager {
	numPieces := uint32(len(tor.Info.Pieces))
	pieceLen := uint32(tor.Info.PieceLen)
	pieces := tor.Info.Pieces
	fileSize := tor.FileSize()

	dm := NewDataManager(pieceLen, fileSize, numPieces, pieces)

	return &PieceManager{
		DataMgr:      dm,
		PcState:      make(map[uint32]*PieceState),
		Storage:      fs,
		FailedPieces: make(map[uint32]int),
		BannedUntil:  make(map[uint32]time.Time),
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

func (pm *PieceManager) SelectBlock(bitfield []byte) (uint32, uint32, uint32, bool) {
	pm.mu.Lock()
	defer func() {
		pm.mu.Unlock()
	}()

	return pm.selectBlock(bitfield)
}

func (pm *PieceManager) selectBlock(bitfield []byte) (uint32, uint32, uint32, bool) {
	for i := uint32(0); i < pm.DataMgr.NumPieces; i++ {
		if isBitInBitfield(i, pm.DataMgr.Completed) || !PeerHasPiece(bitfield, i) || pm.isBanned(i) {
			continue
		}

		ps := pm.PcState[i]

		if ps == nil {
			ps = NewPieceState(i, pm.pieceSize(i))
			ps.Status = Missing
			pm.PcState[i] = ps
		}

		if ps.Status == Verifying || ps.Status == Complete {
			continue
		}

		offset, length, ok := ps.NextBlockToRequest()
		if ok {
			if ps.Status == Missing {
				ps.Status = InProgress
			}
			return i, offset, length, true
		}
	}

	return 0, 0, 0, false
}

func (pm *PieceManager) FinishPiece(index uint32, bitfield []byte, verified bool) (*Message, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	ps := pm.PcState[index]

	if !verified {
		if ps != nil {
			ps.Reset()
		}

		_, ok := pm.handleFailure(index, bitfield)
		if !ok {
			return nil, nil
		}

		return pm.selectNextMessage(bitfield)
	}

	if ps != nil {
		ps.Status = Complete
	}

	pm.markComplete(index)
	delete(pm.PcState, index)

	if pm.isComplete() {
		fmt.Printf("Download complete!\n")
		return nil, nil
	}

	return pm.selectNextMessage(bitfield)
}

func (pm *PieceManager) selectNextMessage(bitfield []byte) (*Message, error) {
	if nextIndex, nextOffset, nextLength, ok := pm.selectBlock(bitfield); ok {
		return pm.prepareRequest(nextIndex, nextLength, nextOffset)
	}

	return nil, nil
}

func (pm *PieceManager) handleFailure(index uint32, bitfield []byte) (uint32, bool) {
	// clear recent piece data from memory
	pieceStart := uint64(index) * uint64(pm.DataMgr.PieceLength)
	pieceSize := uint64(pm.DataMgr.PieceSize(index))
	pm.DataMgr.mu.Lock()
	// zero out piece data
	for i := uint64(0); i < uint64(pieceSize); i++ {
		pm.DataMgr.Data[pieceStart+i] = 0
	}
	pm.DataMgr.mu.Unlock()

	pm.FailedPieces[index]++
	failCount := pm.FailedPieces[index]

	if failCount >= 3 {
		pm.BannedUntil[index] = time.Now().Add(time.Minute * 5)
		fmt.Printf("!!! piece %d failed %d times, bannin temporarily\n", index, failCount)
	}

	delete(pm.PcState, index)

	nextIndex, _, _, ok := pm.selectBlock(bitfield)
	return nextIndex, ok
}

func (pm *PieceManager) prepareRequest(index uint32, length uint32, offset uint32) (*Message, error) {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], offset)
	binary.BigEndian.PutUint32(payload[8:12], length)

	if _, ok := pm.PcState[index]; !ok {
		return nil, fmt.Errorf("piece state not found for index %d", index)
	} /*else if !ps.MarkRequested(offset) {
		return nil, nil
	}*/

	return NewMessageWP(13, Request, payload), nil
}

func (pm *PieceManager) markComplete(index uint32) {
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

	return pm.isComplete()
}

func (pm *PieceManager) isComplete() bool {
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

	return pm.pieceSize(index)
}

func (pm *PieceManager) pieceSize(index uint32) uint32 {
	if index == pm.DataMgr.NumPieces-1 {
		rem := pm.DataMgr.TotalLength % uint64(pm.DataMgr.PieceLength)
		if rem != 0 {
			return uint32(rem)
		}
	}
	return pm.DataMgr.PieceLength
}

func (pm *PieceManager) HandleUnchoke(bitfield []byte) (uint32, uint32, uint32, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	index, offset, length, ok := pm.selectBlock(bitfield)

	return index, offset, length, ok
}

func (pm *PieceManager) HandlePieceMessage(msg *Message) (uint32, bool) {
	pieceIndex := binary.BigEndian.Uint32(msg.Payload[0:4])
	offset := binary.BigEndian.Uint32(msg.Payload[4:8])
	blockData := msg.Payload[8:]

	fmt.Printf("piece received: index=%d offset=%d\n", pieceIndex, offset)

	pm.DataMgr.StoreBlock(pieceIndex, offset, blockData)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	ps, ok := pm.PcState[pieceIndex]
	if !ok || ps == nil {
		return pieceIndex, false
	}

	ps.MarkReceived(offset)
	return pieceIndex, ps.Status == Verifying
}

func (pm *PieceManager) CancelPeerRequest(bitfield []byte) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, ps := range pm.PcState {
		for i := range ps.Blocks {
			if ps.Blocks[i].Requested && !ps.Blocks[i].Received {
				ps.Blocks[i].Requested = false
			}
		}
	}
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

func (pm *PieceManager) isBanned(index uint32) bool {
	until, ok := pm.BannedUntil[index]
	if !ok {
		return false
	}

	if time.Now().Before(until) {
		return true
	}

	delete(pm.BannedUntil, index)
	return false
}

func (pm *PieceManager) MissingPieces() []uint32 {
	pm.DataMgr.mu.RLock()
	defer pm.DataMgr.mu.RUnlock()

	missing := []uint32{}
	for i := range pm.DataMgr.NumPieces {
		if !isBitInBitfield(i, pm.DataMgr.Completed) {
			missing = append(missing, i)
		}
	}

	return missing
}

func (pm *PieceManager) PrintMissingPieces() {
	missing := pm.MissingPieces()

	fmt.Printf("\nmissing %d pieces: %v\n", len(missing), missing)
}
