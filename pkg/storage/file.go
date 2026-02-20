/*
 *
 * title: gotorrent storage file
 * author: Andrew Souza
 * license: GPLv3
 *
 */
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/drewslam/gotorrent/pkg/torrent"
)

type FileStorage struct {
	BaseDir     string
	Files       []*FileInfo
	TotalLength uint64
	PieceLength uint32
	mu          sync.Mutex
}

type FileInfo struct {
	Path   string
	Length uint64
	Offset uint64
	//	Handle *os.File
}

func NewFileStorage(tor *torrent.Torrent, basePath string) (*FileStorage, error) {
	var outputDir string
	if len(tor.Info.Files) == 1 { // single-file
		outputDir = basePath
	} else { // multi-file
		outputDir = filepath.Join(basePath, tor.Info.Name)
	}

	var fileList []*FileInfo
	var currentOffset uint64 = 0

	for _, file := range tor.Info.Files {
		relativePath := filepath.Join(file.Path...)
		fullPath := filepath.Join(outputDir, relativePath)

		cleanBase := filepath.Clean(outputDir)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull != cleanBase && !strings.HasPrefix(cleanFull, cleanBase+string(os.PathSeparator)) {
			return nil, fmt.Errorf("path escapes base dir %s", cleanFull)
		}

		fileList = append(fileList, &FileInfo{
			Path:   fullPath,
			Length: uint64(file.Length),
			Offset: currentOffset,
		})

		currentOffset += uint64(file.Length)
	}

	if err := createDirectories(fileList); err != nil {
		return nil, err
	}

	return &FileStorage{
		BaseDir:     basePath,
		Files:       fileList,
		TotalLength: tor.FileSize(),
		PieceLength: uint32(tor.Info.PieceLen),
	}, nil
}

func createDirectories(files []*FileInfo) error {
	createdDirs := make(map[string]bool)

	for _, file := range files {
		dir := filepath.Dir(file.Path)

		if createdDirs[dir] {
			continue
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		createdDirs[dir] = true
	}

	return nil
}

func (fs *FileStorage) Allocate() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	for _, file := range fs.Files {
		handle, err := os.Create(file.Path)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", file.Path, err)
		}

		if err := handle.Truncate(int64(file.Length)); err != nil {
			handle.Close()
			return fmt.Errorf("failed to allocate space for file %s: %w", file.Path, err)
		}

		if err := handle.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %w", file.Path, err)
		}
	}

	return nil
}

func (fs *FileStorage) WritePiece(pieceIndex uint32, data []byte) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	pieceStart := uint64(pieceIndex) * uint64(fs.PieceLength)
	pieceEnd := pieceStart + uint64(len(data))

		// debug print
		fmt.Printf("writing piece %d: pieceStart=%d pieceEnd=%d len(data)=%d\n", pieceIndex, pieceStart, pieceEnd, len(data))

	for _, file := range fs.Files {
		fstart := file.Offset
		fend := fstart + file.Length

		if fend <= uint64(pieceStart) || fstart >= uint64(pieceEnd) {
			continue
		}

		overlapStart := max(pieceStart, fstart)
		overlapEnd := min(pieceEnd, fend)
		overlapLen := overlapEnd - overlapStart

		foffset := overlapStart - fstart

		boffset := overlapStart - pieceStart

		handle, err := os.OpenFile(file.Path, os.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", file.Path, err)
		}

		_, err = handle.WriteAt(data[boffset:boffset+overlapLen], int64(foffset))
		if err != nil {
			handle.Close()
			return fmt.Errorf("failed to write file %s to disk: %v", file.Path, err)
		}

		// debug print
		fmt.Printf("  file %s: foffset=%d boffset=%d overlapLen=%d\n", file.Path, foffset, boffset, overlapLen)

		if err = handle.Close(); err != nil {
			return fmt.Errorf("failed to close directory: %v", err)
		}
	}

	return nil
}

func (fs *FileStorage) ReadPiece(pieceIndex uint32) ([]byte, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	pieceStart := uint64(pieceIndex) * uint64(fs.PieceLength)
	pieceSize := min(uint64(fs.PieceLength), fs.TotalLength-pieceStart)
	pieceEnd := pieceStart + pieceSize

	buf := make([]byte, pieceSize)

	for _, file := range fs.Files {
		fstart := file.Offset
		fend := fstart + file.Length

		// Check for overlap
		if fend <= pieceStart || fstart >= pieceEnd {
			continue
		}

		// calculate overlap regiop
		overlapStart := max(pieceStart, fstart)
		overlapEnd := min(pieceEnd, fend)
		overlapLen := overlapEnd - overlapStart

		// calculate file offset
		foffset := overlapStart - fstart

		// calculate buffer offset
		boffset := overlapStart - pieceStart

		handle, err := os.Open(file.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", file.Path, err)
		}

		_, err = handle.ReadAt(buf[boffset:boffset+overlapLen], int64(foffset))
		if err != nil {
			handle.Close()
			return nil, fmt.Errorf("failed to read from file %s: %w", file.Path, err)
		}

		if err := handle.Close(); err != nil {
			return nil, fmt.Errorf("failed to close file %s: %w", file.Path, err)
		}

	}

	return buf, nil
}

func min(a uint64, b uint64) uint64 {
	if a < b {
		return a
	}

	return b
}

func max(a uint64, b uint64) uint64 {
	if a > b {
		return a
	}

	return b
}
