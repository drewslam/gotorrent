/*
 *
 * title: gotorrent peer_protocol piecestate
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package peer_protocol

import "fmt"

type BlockState struct {
	Offset    uint32
	Length    uint32
	Requested bool
	Received  bool
}

type PieceState struct {
	Index  uint32
	Blocks []BlockState
}

func NewBlockState(offset uint32, length uint32) BlockState {
	return BlockState{
		Offset:    offset,
		Length:    length,
		Requested: false,
		Received:  false,
	}
}

func NewPieceState(index uint32, pieceSize uint32) *PieceState {
	numBlocks := (pieceSize + MaxBlockSize - 1) / MaxBlockSize
	blocks := make([]BlockState, numBlocks)

	for i := range numBlocks {
		offset := i * MaxBlockSize
		length := MaxBlockSize

		if i == numBlocks-1 {
			length = pieceSize - offset
		}

		blocks[i] = NewBlockState(offset, length)
	}

	return &PieceState{
		Index:  index,
		Blocks: blocks,
	}
}

func (ps *PieceState) AddBlock(offset uint32, data []byte) bool {
	for i := range ps.Blocks {
		if ps.Blocks[i].Offset == offset && !ps.Blocks[i].Received {
			fmt.Printf("marking offset=%v as received\n", offset)
			ps.Blocks[i].Received = true
			return true
		}
	}
	fmt.Printf("failed to find block with offset %d in piece blocks\n", offset)
	return false
}

func (ps *PieceState) NextBlockToRequest() (uint32, uint32, bool) {
	for i := range len(ps.Blocks) {
		fmt.Printf("Block[%d]: offset=%d Requested=%t Received=%t\n", i, ps.Blocks[i].Offset, ps.Blocks[i].Requested, ps.Blocks[i].Received)
		if !ps.Blocks[i].Received && !ps.Blocks[i].Requested {
			ps.Blocks[i].Requested = true
			return ps.Blocks[i].Offset, ps.Blocks[i].Length, true
			// return uint32(i), ps.Blocks[i].Offset, true
		}
	}

	return 0, 0, false
}

func (ps *PieceState) MarkRequested(offset uint32) bool {
	for i := range ps.Blocks {
		if ps.Blocks[i].Offset == offset {
			if !ps.Blocks[i].Requested {
				ps.Blocks[i].Requested = true
				return true
			}
			return false
		}
	}

	return false
}

func (ps *PieceState) IsComplete() bool {
	for i := range ps.Blocks {
		if !ps.Blocks[i].Received {
			return false
		}
	}

	return true
}
