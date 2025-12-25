/*
 *
 *  title: torrent-go
 *  author: Andrew Souza
 *  GPLv3
 *
 */

package torrent

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

type Torrent struct {
	Info      *InfoDict
	Announce  []string
	InfoBytes []byte
}

type InfoDict struct {
	Name     string
	PieceLen int
	Pieces   [][20]byte
	Files    []*FileDict
	UrlList  []string
}

type FileDict struct {
	Length int
	Path   []string
}

func NewTorrent(info *InfoDict, announce []string, byts []byte) *Torrent {
	return &Torrent{
		Info:      info,
		Announce:  announce,
		InfoBytes: byts,
	}
}

func NewInfoDict(name string, pieceLen int, pieces [][20]byte, fileDict []*FileDict, urlList []string) *InfoDict {
	return &InfoDict{
		Name:     name,
		PieceLen: pieceLen,
		Pieces:   pieces,
		Files:    fileDict,
		UrlList:  urlList,
	}
}

func NewFileDict(length int, path []string) *FileDict {
	return &FileDict{
		Length: length,
		Path:   path,
	}
}

func extractPieces(pieces []byte) ([][20]byte, error) {
	if len(pieces)%20 != 0 {
		return nil, fmt.Errorf("invalid piece length: %d", len(pieces))
	}

	pieceCount := len(pieces)/20
	pc := make([][20]byte, pieceCount)

	for i := range pieceCount {
		copy(pc[i][:], pieces[i*20:(i+1)*20])
	}

	return pc, nil
}

func extractAnnounceList(list *bcodec.BDictEntry, entry *bcodec.BDictEntry) ([]string, error) {
	var announceList []string

	if list != nil {
		outer, ok := bcodec.AsListNode(list.Value)
		if !ok {
			return nil, fmt.Errorf("cannot parse node: %v", outer)
		}

		for _, inner := range outer.GetChildren() {
			item, k := bcodec.AsListNode(inner)
			if !k {
				return nil, fmt.Errorf("cannot parse node: %v", inner)
			}

			for _, url := range item.GetChildren() {
				urlv, y := bcodec.AsValueNode(url)
				if !y {
					return nil, fmt.Errorf("cannot parse node: %v", urlv)
				}

				announceList = append(announceList, string(urlv.GetValue().Strval))
			}
		}
	} else if entry != nil {
		node, ok := bcodec.AsValueNode(entry.Value)
		if !ok {
			return nil, fmt.Errorf("cannot parse node: %v", node)
		}

		announceList = []string{string(node.GetValue().Strval)}
	} else {
		return nil, fmt.Errorf("torrent missing announce information")
	}

	return announceList, nil
}

func extractUrlList(node *bcodec.BDictEntry) ([]string, error) {
	if node == nil {
		return nil, nil
	}

	var urlList []string

	list, ok := bcodec.AsListNode(node.Value)

	if !ok {
		return nil, fmt.Errorf("cannot parse node: %v", list)
	}

	for _, url := range list.GetChildren() {
		if urlv, _ := bcodec.AsValueNode(url); urlv != nil {
			urlList = append(urlList, string(urlv.GetValue().Strval))
		}
	}

	return urlList, nil
}

func extractPieceArray(node *bcodec.BDictEntry) ([]byte, error) {
	if node == nil {
		return nil, fmt.Errorf("piece array cannot be parsed: %v", node)
	}

	a, ok := bcodec.AsValueNode(node.Value)
	if !ok {
		return nil, fmt.Errorf("cannot parse node: %v", a)
	}

	return a.GetValue().Strval, nil
}

func extractPieceLength(node *bcodec.BDictEntry) (int, error) {
	if node == nil {
		return 0, fmt.Errorf("piece length cannot be parsed: %v", node)
	}

	a, ok := bcodec.AsValueNode(node.Value)
	if !ok {
		return 0, fmt.Errorf("cannot parse node: %v", a)
	}

	return int(a.GetValue().Big_ival), nil
}

func extractFileName(node *bcodec.BDictEntry) (string, error) {
	if node == nil {
		return "", fmt.Errorf("name cannot be parsed: %v", node)
	}

	a, ok := bcodec.AsValueNode(node.Value)
	if !ok {
		return "", fmt.Errorf("cannot parse node: %v", a)
	}

	return string(a.GetValue().Strval), nil
}

func extractInfoBytes(node *bcodec.BDictEntry, rawBytes []byte) ([]byte, error) {
	if node == nil || rawBytes == nil {
		return nil, fmt.Errorf("info structure not present")
	}

	start := node.Value.GetOffset()
	end := start + node.Value.GetLength()

	return rawBytes[start:end], nil
}

func extractSingleFileLength(node *bcodec.BDictEntry) (int, error) {
	if node == nil {
		return 0, fmt.Errorf("length field missing")
	}

	a, ok := bcodec.AsValueNode(node.Value)
	if !ok {
		return 0, fmt.Errorf("invalid length property: %v", a)
	}

	return int(a.GetValue().Big_ival), nil
}

func parseFileList(list *bcodec.BDictEntry, entry *bcodec.BDictEntry, entryName string) ([]*FileDict, error) {
	var fileList []*FileDict

	if list != nil {
		node, ok := bcodec.AsListNode(list.Value)
		if !ok {
			return nil, fmt.Errorf("cannot parse list node: %v", node)
		}

		for _, file := range node.GetChildren() {
			fd, ok := bcodec.AsDictNode(file)
			if !ok {
				return nil, fmt.Errorf("unable to parse file path: %v", fd)
			}

			ln := fd.FindEntry([]byte("length"))
			fp := fd.FindEntry([]byte("path"))

			if ln == nil {
				return nil, fmt.Errorf("unable to parse length: %v", ln)
			}
			if fp == nil {
				return nil, fmt.Errorf("unable to parse file path: %v", fp)
			}

			lv, err := extractSingleFileLength(ln)
			if err != nil {
				return nil, fmt.Errorf("unable to parse length value: %v", err)
			}

			pv, ok := bcodec.AsListNode(fp.Value)
			if !ok {
				return nil, fmt.Errorf("unable to parse path value: %v", pv)
			}

			var filePath []string
			for _, inner := range pv.GetChildren() {
				no, ok := bcodec.AsValueNode(inner)
				if !ok {
					return nil, fmt.Errorf("invalid path string: %v", no)
				}

				filePath = append(filePath, string(no.GetValue().Strval))
			}

			fileList = append(fileList, NewFileDict(lv, filePath))
		}
	} else if entry != nil {
		length, err := extractSingleFileLength(entry)
		if err != nil {
			return nil, fmt.Errorf("extractSingleFileLength failure: %w", err)
		}

		filePath := []string{entryName}

		fileList = append(fileList, NewFileDict(length, filePath))
	} else {
		return nil, fmt.Errorf("malformed data")
	}

	return fileList, nil
}

func extractInfoDict(entry *bcodec.BDictEntry, rawBytes []byte, urlList []string) (*InfoDict, []byte, error) {
	if entry == nil {
		return nil, nil, fmt.Errorf("contents of info dictionary cannot be parsed")
	}

	infoHash, err := extractInfoBytes(entry, rawBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("extractInfoBytes failure: %v", err)
	}

	id, ok := bcodec.AsDictNode(entry.Value)
	if !ok {
		return nil, nil, fmt.Errorf("invalid 'info' field: %v", id)
	}

	nm := id.FindEntry([]byte("name"))
	pl := id.FindEntry([]byte("piece length"))
	pc := id.FindEntry([]byte("pieces"))
	ln := id.FindEntry([]byte("length"))
	fi := id.FindEntry([]byte("files"))

	name, err := extractFileName(nm)
	if err != nil {
		return nil, nil, fmt.Errorf("extractFileName failure: %v", err)
	}

	pieceLen, err := extractPieceLength(pl)
	if err != nil {
		return nil, nil, fmt.Errorf("extractPieceLength failure: %v", err)
	}

	pieces, err := extractPieceArray(pc)
	if err != nil {
		return nil, nil, fmt.Errorf("extractPieceArray failure: %v", err)
	}

	fileList, err := parseFileList(fi, ln, name)
	if err != nil {
		return nil, nil, fmt.Errorf("torrent must have either 'length' or 'files' field")
	}

	exp, err := extractPieces(pieces)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract bytes from pieces: %v", err)
	}

	return NewInfoDict(name, pieceLen, exp, fileList, urlList), infoHash, nil
}

func ParseMetadata(torrentFile *bcodec.BDictNode, rawBytes []byte) (*Torrent, error) {
	al := torrentFile.FindEntry([]byte("announce-list"))
	an := torrentFile.FindEntry([]byte("announce"))
	in := torrentFile.FindEntry([]byte("info"))
	ul := torrentFile.FindEntry([]byte("url-list"))

	announceList, err := extractAnnounceList(al, an)
	if err != nil {
		return nil, fmt.Errorf("extractAnnounceList failure: %w", err)
	}

	urlList, err := extractUrlList(ul)
	if err != nil {
		return nil, fmt.Errorf("extractUrlList failure: %w", err)
	}

	infoDict, infoHash, err := extractInfoDict(in, rawBytes, urlList)
	if err != nil {
		return nil, fmt.Errorf("extractInfoDict failure: %w", err)
	}

	return NewTorrent(infoDict, announceList, infoHash), nil
}

func (t *Torrent) InfoHash() [20]byte {
	return sha1.Sum(t.InfoBytes)
}

func (t *Torrent) FileSize() uint64 {
	if t == nil {
		return 0
	}

	var fs uint64 = 0
	for _, file := range t.Info.Files {
		fs += uint64(file.Length)
	}

	return fs
}

func (t *Torrent) PrintMetadata() {
	fmt.Printf("announce: %s\npiece length: %d\n", t.Announce[0], t.Info.PieceLen)
	fmt.Printf("name: %s\n", t.Info.Name)

	if len(t.Info.Files) > 0 {
		for c, i := range t.Info.Files {
			fmt.Printf("%d - %s - %d\n", c, strings.Join(i.Path, "/"), i.Length)
		}
	}

}
