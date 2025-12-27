# gotorrent

A BitTorrent library for Go focused on feature-completeness and low resource usage.

[![img](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![img](https://img.shields.io/badge/Go-%3E%3D%201.22-blue)](https://go.dev/)

## Development Status

This library is in **early development**. Core components (bencode, metadata parsing, tracker communication) are functional, but the peer wire protocol and download management are not yet implemented.

## Features

### Currently Working

-   ✅ **Bencode encoding/decoding** - Full support for all bencode types
-   ✅ **Torrent file parsing** - Single and multi-file torrents
-   ✅ **Info hash calculation** - Proper SHA-1 of info dictionary
-   ✅ **HTTP/HTTPS trackers** - Complete announce protocol
-   ✅ **UDP trackers** - Connect and announce implementation
-   ✅ **Peer discovery** - Get peer lists from trackers
-   ✅ **Protocol abstraction** - Single interface for HTTP and UDP trackers

### Limitations

-   No retry logic or timeout handling
-   No periodic re-announce scheduling
-   No connection pooling or concurrent tracker requests

### What You Can Do Today
-   Parse any .torrent file (single or multi-file)
-   Calculate info hashes for tracker communication
-   Announce to HTTP/S and UDP trackers
-   Retrieve lists of peers from the swarm
-   Get swarm statistics (seeders, leechers, re-announce interval)

### Not Yet Implemented

-   Peer wire protocol (connecting to/from peers)
-   Piece download/upload logic
-   File I/O and assembly
-   DHT, PEX, magnet links
-   High-level client API

## Architecture

### Packages

**`pkg/bcodec`** - Bencode encoding and decoding

-   Tree-based parser with typed nodes
-   Support for all bencode types (integers, strings, lists, dictionaries)
-   Type-safe casting and visitor pattern

**`pkg/torrent`** - Torrent metadata handling

-   Parse .torrent files
-   Extract info hash, pieces, file information
-   Support for announce-list and url-list

**`pkg/tracker`** - Tracker communication

-   HTTP/HTTPS tracker protocol
-   UDP tracker protocol
-   Peer list retrieval

## Installation

    go get github.com/drewslam/gotorrent

Requires Go 1.22 or later.

## Usage

### Current Capabilities

    // Example of what works today
    import (
        "github.com/drewslam/gotorrent/pkg/bcodec"
        "github.com/drewslam/gotorrent/pkg/torrent"
        "github.com/drewslam/gotorrent/pkg/tracker"
    )

    // Parse a torrent file
    data, _ := os.ReadFile("example.torrent")
    decoder, _ := bcodec.NewBDecoder(data, false, 0)
    dict, _ := decoder.DecodeDict()
    tor, _ := torrent.ParseMetadata(dict, data)

    // Get info hash
    hash := tor.InfoHash()

    // Contact tracker
    peer := tracker.NewPeer()
    req := tracker.NewRequest(hash, peer, tor.FileSize())
    url, _ := url.Parse(tor.Announce[0])
    response, _ := req.Announce(url)

    // response.Peers contains available peers
    fmt.Printf("Found %d peers\n", len(response.Peers))
    fmt.Printf("Re-announce in %d seconds\n", response.Interval)
    fmt.Printf("Swarm: %d seeders, %d leechers\n", response.Seeders, response.Leechers)

## Design Goals

1.  **Feature-complete** - Full BitTorrent protocol support
2.  **Low resource usage** - Efficient memory and CPU utilization
3.  **Library-first** - Clean API for building clients
4.  **Well-tested** - Comprehensive test coverage (in progress)

## Development

### Current Focus

-   ~~Unifying HTTP/UDP response types~~ ✅ Complete
-   Implementing peer wire protocol
-   Building piece verification logic
-   Adding test coverage

## License

GPL-3.0 - See [LICENSE](LICENSE) for details.

## Acknowledgments

-   BitTorrent protocol specification: [BEP 0003](http://www.bittorrent.org/beps/bep_0003.html)
-   [libktorrent](https://github.com/KDE/libktorrent) - Design influence for the bcodec package
