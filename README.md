# gotorrent
A BitTorrent library for Go focused on feature-completeness and low resource usage.

[![img](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![img](https://img.shields.io/badge/Go-%3E%3D%201.23-blue)](https://go.dev/)

## Development Status
This library is in **active development**. Core components (bencode, metadata parsing, tracker communication, peer protocol) are functional. File download management is currently being implemented.

## Features

### Currently Working
-   ✅ **Bencode encoding/decoding** - Full support for all bencode types
-   ✅ **Torrent metadata parsing** - Single and multi-file torrents
-   ✅ **Info hash calculation** - Proper SHA-1 of info dictionary
-   ✅ **HTTP/HTTPS trackers** - Full announce protocol
-   ✅ **UDP trackers** - Connect and announce implementation
-   ✅ **Peer discovery** - Retrieve peer lists from trackers
-   ✅ **Protocol abstraction** - Single interface for HTTP and UDP trackers
-   ✅ **Peer wire protocol** - Handshake, message parsing, and state management
-   ✅ **Concurrent piece coordination** - Block level tracking across multiple peers
-   🚧 **File download** - In progress: block requests, piece assembly, verification

### What You Can Do Today
-   Parse any .torrent file (single or multi-file)
-   Calculate info hashes for tracker communication
-   Announce to HTTP/S and UDP trackers
-   Retrieve lists of peers from the swarm
-   Get swarm statistics (seeders, leechers, re-announce interval)
-   Establish connection with peers via handshake
-   Exchange BitTorrent protocol messages
-   Request and receive piece blocks from peers
-   Track download progress at the block and piece level

### Not Yet Implemented
-   Complete file assembly and writing to disk
-   Upload/seeding functionality
-   DHT, PEX, magnet links
-   Resume support
-   Rate limiting and bandwidth management
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

**`pkg/peer_protocol`** - Peer wire protocol
-   Handshake negotiation and validation
-   Message encoding/decoding
-   Peer connection state management
-   Block-level piece state tracking
-   Concurrent peer coordination via PieceManager
-   Thread-safe data storage with DataManager

## Installation
```bash
    go get github.com/drewslam/gotorrent
```

Requires Go 1.23 or later.

## Usage

### Current Capabilities
```go
    // Example of what works today
    import (
        "github.com/drewslam/gotorrent/pkg/torrent"
        "github.com/drewslam/gotorrent/pkg/tracker"
        "github.com/drewslam/gotorrent/pkg/peer_protocol"
    )

    // Parse a torrent file
    data, _ := os.ReadFile("example.torrent")
    tor, _ := torrent.DecodeTorrentFile(data)

    // Get info hash
    hash := tor.InfoHash()

    // Contact tracker
    peer := tracker.NewPeer()
    req := tracker.NewRequest(hash, peer, tor.FileSize())
    url, _ := url.Parse(tor.Announce[0])
    response, _ := req.Announce(url)

    // Connect to peers and begin download
    pm := peer_protocol.NewPieceManager(tor)
    for index, peer := range response.Peers {
        go peer_protocol.HandleConnection(peer.Address(), index, req, pm)
    }

    // Peers will automatically
    // - Perform handshake
    // - Exchange bitfields
    // - Request and download pieces
    // - Verify piece hashes
```

## Design Goals

1.  **Feature-complete** - Full BitTorrent protocol support
2.  **Low resource usage** - Efficient memory and CPU utilization
3.  **Library-first** - Clean API for building client
4.  **Well-tested** - Comprehensive test coverage (in progress)

## Development

### Current Focus
-   Completing file I/O and piece assembly
-   Adding upload/seeding support
-   Implementing resume capability
-   Adding test coverage

### Architecture Notes
The peer protocol implementation uses a multi-layered architecture:

-   **DataManager**: Thread-sage storage for downloaded data and completion tracking
-   **PieceManager**: Coordinates piece selection and block-level state across multiple peers
-   **PieceState**: Tracks individual block requests and reception for each piece
-   **PeerConn**: Handles network I/O and protocol message exchange with individual peers

This design allows multiple peers to concurrently download different pieces while preventing duplicate work.

## License

GPL-3.0 - See [LICENSE](LICENSE) for details.

## Acknowledgments

-   BitTorrent protocol specification: [BEP 0003](http://www.bittorrent.org/beps/bep_0003.html)
-   [libktorrent](https://github.com/KDE/libktorrent) - Design influence for the bcodec package
-   Building a BitTorrent client from the ground up in Go, by Jesse Li (https://blog.jse.li/posts/torrent/)
