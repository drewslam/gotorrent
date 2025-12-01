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
	Info     *InfoDict
	Announce []string
	Hash     []byte
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

func NewTorrent(info *InfoDict, announce []string, hash []byte) *Torrent {
	return &Torrent{
		Info:     info,
		Announce: announce,
		Hash:     hash,
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

func ExtractPieces(pieces []byte, pieceCount int) ([][20]byte, error) {
	if len(pieces)%20 != 0 {
		return nil, fmt.Errorf("invalid piece length: %d", len(pieces))
	}
	pc := make([][20]byte, pieceCount)
	for i := range pieceCount {
		copy(pc[i][:], pieces[i*20:(i+1)*20])
	}
	return pc, nil
}

func ParseMetadata(torrentFile *bcodec.BDictNode, rawBytes []byte) (*Torrent, error) {
	var announceList []string
	var infoDict *InfoDict
	var infoHash []byte
	var urlList []string

	an := torrentFile.FindEntry([]byte("announce"))
	al := torrentFile.FindEntry([]byte("announce-list"))
	in := torrentFile.FindEntry([]byte("info"))
	ul := torrentFile.FindEntry([]byte("url-list"))

	// announce-list is a list of lists, where each inner list contains an announce url
	if al != nil {
		ol, ok := bcodec.AsListNode(al.Value)
		if !ok {
			return nil, fmt.Errorf("cannot parse node: %v", ol)
		}
		for _, il := range ol.GetChildren() {
			ill, k := bcodec.AsListNode(il)
			if !k {
				return nil, fmt.Errorf("cannot parse node: %v", ill)
			}
			for _, url := range ill.GetChildren() {
				urlv, y := bcodec.AsValueNode(url)
				if !y {
					return nil, fmt.Errorf("cannot parse node: %v", urlv)
				}
				announceList = append(announceList, string(urlv.GetValue().Strval))
			}
		}
	} else if an != nil {
		node, ok := bcodec.AsValueNode(an.Value)
		if !ok {
			return nil, fmt.Errorf("cannot parse node: %v", node)
		}
		announceList = []string{string(node.GetValue().Strval)}
	} else {
		announceList = []string{""}
	}

	if ul != nil {
		list, ok := bcodec.AsListNode(ul.Value)
		if !ok {
			return nil, fmt.Errorf("cannot parse node: %v", list)
		}
		for _, url := range list.GetChildren() {
			if urlv, _ := bcodec.AsValueNode(url); urlv != nil {
				urlList = append(urlList, string(urlv.GetValue().Strval))
			}
		}
	}

	if in == nil {
		return nil, fmt.Errorf("contents of info dictionary cannot be parsed")
	} else {
		start := in.Value.GetOffset()
		end := start + in.Value.GetLength()
		infoHash = rawBytes[start:end]
		id, ok := bcodec.AsDictNode(in.Value)
		if !ok {
			return nil, fmt.Errorf("cannot parse node: %v", id)
		}

		var name string
		var pieceLen int
		var pieces []byte
		var length int

		nm := id.FindEntry([]byte("name"))
		pl := id.FindEntry([]byte("piece length"))
		pc := id.FindEntry([]byte("pieces"))
		ln := id.FindEntry([]byte("length"))
		fi := id.FindEntry([]byte("files"))

		if nm == nil {
			return nil, fmt.Errorf("name cannot be parsed: %v", nm)
		} else {
			node, ok := bcodec.AsValueNode(nm.Value)
			if !ok {
				return nil, fmt.Errorf("cannot parse node: %v", node)
			}
			name = string(node.GetValue().Strval)
		}

		if pl == nil {
			return nil, fmt.Errorf("piece length cannot be parsed: %v", pl)
		} else {
			node, ok := bcodec.AsValueNode(pl.Value)
			if !ok {
				return nil, fmt.Errorf("cannot parse node: %v", node)
			}
			pieceLen = int(node.GetValue().Big_ival)
		}

		if pc == nil {
			return nil, fmt.Errorf("piece array cannot be parsed: %v", pc)
		} else {
			node, ok := bcodec.AsValueNode(pc.Value)
			if !ok {
				return nil, fmt.Errorf("cannot parse node: %v", node)
			}
			pieces = node.GetValue().Strval
		}

		var fileList []*FileDict
		if ln != nil {
			node, ok := bcodec.AsValueNode(ln.Value)
			if !ok {
				return nil, fmt.Errorf("cannot parse node: %v", node)
			}
			length = int(node.GetValue().Big_ival)
			filePath := []string{name}
			fileList = append(fileList, NewFileDict(length, filePath))
		} else if fi != nil {
			node, ok := bcodec.AsListNode(fi.Value)
			if !ok {
				return nil, fmt.Errorf("cannot parse node: %v", node)
			}
			for _, i := range node.GetChildren() {
				fd, ok := bcodec.AsDictNode(i)
				if !ok {
					return nil, fmt.Errorf("unable to parse file path: %v", fd)
				}

				lno := fd.FindEntry([]byte("length"))
				fp := fd.FindEntry([]byte("path"))

				if lno == nil {
					return nil, fmt.Errorf("unable to parse length: %v", lno)
				}
				if fp == nil {
					return nil, fmt.Errorf("unable to parse file path: %v", fp)
				}

				lv, ok := bcodec.AsValueNode(lno.Value)
				if !ok {
					return nil, fmt.Errorf("unable to parse length value: %v", lv)
				}

				pv, ok := bcodec.AsListNode(fp.Value)
				if !ok {
					return nil, fmt.Errorf("unable to parse path value: %v", pv)
				}
				var filePath []string
				for _, j := range pv.GetChildren() {
					no, ok := bcodec.AsValueNode(j)
					if !ok {
						return nil, fmt.Errorf("cannot parse node: %v", node)
					}
					filePath = append(filePath, string(no.GetValue().Strval))
				}
				fileDict := NewFileDict(int(lv.GetValue().Big_ival), filePath)
				fileList = append(fileList, fileDict)
			}
		} else {
			return nil, fmt.Errorf("torrent must have either 'length' or 'files' field")
		}

		exp, err := ExtractPieces(pieces, len(pieces)/20)
		if err != nil {
			return nil, fmt.Errorf("failed to extract bytes from pieces: %v", err)
		}
		infoDict = NewInfoDict(name, pieceLen, exp, fileList, urlList)
	}

	return NewTorrent(infoDict, announceList, infoHash), nil
}

func (t *Torrent) InfoHash() [20]byte {
	return sha1.Sum(t.Hash)
}

func (t *Torrent) FileSize() uint64 {
	if t == nil { return 0 }

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
