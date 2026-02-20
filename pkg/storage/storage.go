/*
 *
 * title: gotorrent storage
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package storage

type Storage interface {
	WritePiece(pieceIndex uint32, data []byte) error
	ReadPiece(pieceIndex uint32) ([]byte, error)
	Allocate() error
	Close() error
}
