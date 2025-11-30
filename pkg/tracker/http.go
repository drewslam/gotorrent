/*
 * title: gotorrent-tracker http
 * author: Andrew Souza
 * GPLv3
 */

package tracker

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/drewslam/gotorrent/pkg/bcodec"
)

// URL Processer
func (r *Request) URL(announce string) string {
	port := strconv.Itoa(r.Peer.Port)
	uploaded := strconv.FormatUint(r.Uploaded, 10)
	downloaded := strconv.FormatUint(r.Downloaded, 10)
	left := strconv.FormatUint(r.Left, 10)
	peerId := EscapeBytes(r.Peer.ID)
	infoHash := EscapeBytes(r.InfoHash[:])
	return announce + "?info_hash=" + infoHash +
		"&peer_id=" + peerId +
		"&uploaded=" + uploaded +
		"&downloaded=" + downloaded +
		"&left=" + left +
		"&event=" + r.Event +
		"&compact=1" +
		"&port=" + port
}

func (r *Request) FetchHttpResponse(announce string) (*Response, error) {
	ur := r.URL(announce)

	resp, err := http.Get(ur)
	if err != nil {
		return nil, fmt.Errorf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	rs, err := DecodeResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tracker response: %v", err)
	}
	return rs, nil
}

func DecodeResponse(resp *http.Response) (*Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	db, err := bcodec.NewBDecoder(body, false, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bdecoder: %v", err)
	}
	dbc, err := db.DecodeDict()
	if err != nil {
		return nil, fmt.Errorf("failed to decoder response: %v", err)
	}

	return NewResponse(dbc.Children), nil
}
