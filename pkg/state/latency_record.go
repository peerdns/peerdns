// pkg/state/latency_record.go
package state

import (
	"bytes"
	"encoding/binary"

	"github.com/klauspost/compress/s2"
)

// LatencyRecord tracks the latency metrics for a peer.
type LatencyRecord struct {
	PeerID    string // Unique identifier for the peer
	LatencyMs int64  // Latency in milliseconds
	Timestamp int64  // Unix timestamp
}

// Encode serializes and compresses the LatencyRecord.
func (lr *LatencyRecord) Encode() ([]byte, error) {
	buf := &bytes.Buffer{}

	// Write PeerID length and PeerID
	if err := binary.Write(buf, binary.LittleEndian, int32(len(lr.PeerID))); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(lr.PeerID); err != nil {
		return nil, err
	}

	// Write LatencyMs
	if err := binary.Write(buf, binary.LittleEndian, lr.LatencyMs); err != nil {
		return nil, err
	}

	// Write Timestamp
	if err := binary.Write(buf, binary.LittleEndian, lr.Timestamp); err != nil {
		return nil, err
	}

	// Compress using s2
	compressed := s2.Encode(nil, buf.Bytes())
	return compressed, nil
}

// DecodeLatencyRecord decompresses and deserializes the LatencyRecord.
func DecodeLatencyRecord(data []byte) (*LatencyRecord, error) {
	// Decompress using s2
	decompressed, err := s2.Decode(nil, data)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewReader(decompressed)
	record := &LatencyRecord{}

	// Read PeerID length and PeerID
	var peerIDLen int32
	if err := binary.Read(buf, binary.LittleEndian, &peerIDLen); err != nil {
		return nil, err
	}
	peerIDBytes := make([]byte, peerIDLen)
	if _, err := buf.Read(peerIDBytes); err != nil {
		return nil, err
	}
	record.PeerID = string(peerIDBytes)

	// Read LatencyMs
	if err := binary.Read(buf, binary.LittleEndian, &record.LatencyMs); err != nil {
		return nil, err
	}

	// Read Timestamp
	if err := binary.Read(buf, binary.LittleEndian, &record.Timestamp); err != nil {
		return nil, err
	}

	return record, nil
}
