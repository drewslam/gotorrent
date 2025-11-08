/*
 *
 *  title: torrent-go
 *  author: Andrew Souza
 *  GPLv3
 *
 */

package torrent

import (
	"fmt"
	"log"
	"strings"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

type Torrent struct {
	Info     *InfoDict
	Announce string
}

type InfoDict struct {
	Name     string
	PieceLen int
	Pieces   [][20]byte
	Files    []*FileDict
}

type FileDict struct {
	Length int
	Path   []string
}

func NewTorrent(info *InfoDict, announce string) *Torrent {
	return &Torrent{
		Info:     info,
		Announce: announce,
	}
}

func NewInfoDict(name string, pieceLen int, pieces [][20]byte, fileDict []*FileDict) *InfoDict {
	return &InfoDict{
		Name:     name,
		PieceLen: pieceLen,
		Pieces:   pieces,
		Files: fileDict,
	}
}

func NewFileDict(length int, path []string) *FileDict {
	return &FileDict{
		Length: length,
		Path: path,
	}
}

func ExtractPieces(pieces []byte, pieceCount int) [][20]byte {
	if len(pieces)%20 != 0 {
		log.Fatalf("invalid piece length: %d", len(pieces))
	}
	pc := make([][20]byte, pieceCount)
	for i := 0; i < pieceCount; i++ {
		copy(pc[i][:], pieces[i*20:(i+1)*20])
	}
	return pc
}

func ParseMetadata(torrentFile *bcodec.BDictNode) *Torrent {
	var announce string
	var infoDict *InfoDict

	an := torrentFile.FindEntry([]byte("announce"))
	in := torrentFile.FindEntry([]byte("info"))

	if an == nil {
		announce = ""
	} else {
		node, ok := bcodec.AsValueNode(an.Value)
		if !ok {
			log.Fatalf("cannot parse node: %v", node)
		}
		announce = string(node.GetValue().Strval)
	}

	if in == nil {
		log.Fatalf("contents of info dictionary cannot be parsed")
	} else {
		id, ok := bcodec.AsDictNode(in.Value)
		if !ok {
			log.Fatalf("cannot parse node: %v", id)
		}

		var name string
		var pieceLen int
		var pieces []byte
		var length int

		for _, i := range id.GetEntries() {
			fmt.Printf("%s\n", i.Key)
		}
		fmt.Println("--------------------------")

		nm := id.FindEntry([]byte("name"))
		pl := id.FindEntry([]byte("piece length"))
		pc := id.FindEntry([]byte("pieces"))
		ln := id.FindEntry([]byte("length"))
		fi := id.FindEntry([]byte("files"))

		if nm == nil {
			log.Fatalf("name cannot be parsed: %v", nm)
		} else {
			node, ok := bcodec.AsValueNode(nm.Value)
			if !ok {
				log.Fatalf("cannot parse node: %v", node)
			}
			name = string(node.GetValue().Strval)
		}

		if pl == nil {
			log.Fatalf("piece length cannot be parsed: %v", pl)
		} else {
			node, ok := bcodec.AsValueNode(pl.Value)
			if !ok {
				log.Fatalf("cannot parse node: %v", node)
			}
			pieceLen = int(node.GetValue().Big_ival)
		}

		if pc == nil {
			log.Fatalf("piece array cannot be parsed: %v", pc)
		} else {
			node, ok := bcodec.AsValueNode(pc.Value)
			if !ok {
				log.Fatalf("cannot parse node: %v", node)
			}
			pieces = node.GetValue().Strval
		}

		if ln == nil {
			fmt.Println("no length header present: multi-file mode")
		} else {
			node, ok := bcodec.AsValueNode(ln.Value)
			if !ok {
				log.Fatalf("cannot parse node: %v", node)
			}
			length = int(node.GetValue().Big_ival)
		}

		var fileList []*FileDict
		if fi != nil {
			node, ok := bcodec.AsListNode(fi.Value)
			if !ok {
				log.Fatalf("cannot parse node: %v", node)
			}
			for _, i := range node.GetChildren() {
				fd, ok := bcodec.AsDictNode(i)
				if !ok {
					log.Fatalf("unable to parse file path: %v", fd)
				}

				lno := fd.FindEntry([]byte("length"))
				fp := fd.FindEntry([]byte("path"))

				if lno == nil {
					log.Fatalf("unable to parse length: %v", lno)
				}
				if fp == nil {
					log.Fatalf("unable to parse file path: %v", fp)
				}

				lv, ok := bcodec.AsValueNode(lno.Value)
				if !ok {
					log.Fatalf("unable to parse length value: %v", lv)
				}

				pv, ok := bcodec.AsListNode(fp.Value)
				if !ok {
					log.Fatalf("unable to parse path value: %v", pv)
				}
				var filePath []string
				for _, j := range pv.GetChildren() {
					node, ok := bcodec.AsValueNode(j)
					if !ok {
						log.Fatalf("cannot parse node: %v", node)
					}
					filePath = append(filePath, string(node.GetValue().Strval))
				}
				fileDict := NewFileDict(int(lv.GetValue().Big_ival), filePath)
				fileList = append(fileList, fileDict)
			}
		} else {
			filePath := []string{name}
			fileList = append(fileList, NewFileDict(length, filePath))
		}

		infoDict = NewInfoDict(name, pieceLen, ExtractPieces(pieces, len(pieces)/20), fileList)
	}


	return NewTorrent(infoDict, announce);
}

func (t *Torrent) PrintMetadata() {
	fmt.Printf("announce: %s\npiece length: %d\n", t.Announce, t.Info.PieceLen)
	fmt.Printf("name: %s\n", t.Info.Name)

	if len(t.Info.Files) > 0 {
		for c, i := range t.Info.Files {
			fmt.Printf("%d - %s - %d\n", c, strings.Join(i.Path, "/"), i.Length)
		}
	}
}
