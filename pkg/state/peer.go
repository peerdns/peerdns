// pkg/state/peer.go
package state

import (
	"bytes"
	"encoding/binary"
	"github.com/klauspost/compress/s2"
)

// Peer represents a network peer.
type Peer struct {
	ID       string // Unique identifier for the peer
	Address  string // Network address
	JoinedAt int64  // Unix timestamp
	LeftAt   *int64 // Unix timestamp, nil if active
}

// Encode serializes and compresses the Peer.
func (p *Peer) Encode() ([]byte, error) {
	buf := &bytes.Buffer{}

	// Write ID length and ID
	if err := binary.Write(buf, binary.LittleEndian, int32(len(p.ID))); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(p.ID); err != nil {
		return nil, err
	}

	// Write Address length and Address
	if err := binary.Write(buf, binary.LittleEndian, int32(len(p.Address))); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(p.Address); err != nil {
		return nil, err
	}

	// Write JoinedAt
	if err := binary.Write(buf, binary.LittleEndian, p.JoinedAt); err != nil {
		return nil, err
	}

	// Write LeftAt flag and value
	if p.LeftAt != nil {
		if err := binary.Write(buf, binary.LittleEndian, byte(1)); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.LittleEndian, *p.LeftAt); err != nil {
			return nil, err
		}
	} else {
		if err := binary.Write(buf, binary.LittleEndian, byte(0)); err != nil {
			return nil, err
		}
	}

	// Compress using s2
	compressed := s2.Encode(nil, buf.Bytes())
	return compressed, nil
}

// DecodePeer decompresses and deserializes the Peer.
func DecodePeer(data []byte) (*Peer, error) {
	// Decompress using s2
	decompressed, err := s2.Decode(nil, data)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewReader(decompressed)
	peer := &Peer{}

	// Read ID length and ID
	var idLen int32
	if err := binary.Read(buf, binary.LittleEndian, &idLen); err != nil {
		return nil, err
	}
	idBytes := make([]byte, idLen)
	if _, err := buf.Read(idBytes); err != nil {
		return nil, err
	}
	peer.ID = string(idBytes)

	// Read Address length and Address
	var addrLen int32
	if err := binary.Read(buf, binary.LittleEndian, &addrLen); err != nil {
		return nil, err
	}
	addrBytes := make([]byte, addrLen)
	if _, err := buf.Read(addrBytes); err != nil {
		return nil, err
	}
	peer.Address = string(addrBytes)

	// Read JoinedAt
	if err := binary.Read(buf, binary.LittleEndian, &peer.JoinedAt); err != nil {
		return nil, err
	}

	// Read LeftAt flag and value
	var flag byte
	if err := binary.Read(buf, binary.LittleEndian, &flag); err != nil {
		return nil, err
	}
	if flag == 1 {
		var leftAt int64
		if err := binary.Read(buf, binary.LittleEndian, &leftAt); err != nil {
			return nil, err
		}
		peer.LeftAt = &leftAt
	} else {
		peer.LeftAt = nil
	}

	return peer, nil
}
