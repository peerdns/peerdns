// pkg/state/finalization.go
package state

import (
	"bytes"
	"encoding/binary"

	"github.com/klauspost/compress/s2"
)

// Finalization represents the finalization of a block.
type Finalization struct {
	BlockHash []byte // Hash of the finalized block
	Timestamp int64  // Unix timestamp
}

// Encode serializes and compresses the Finalization.
func (f *Finalization) Encode() ([]byte, error) {
	buf := &bytes.Buffer{}

	// Write BlockHash length and BlockHash
	if err := binary.Write(buf, binary.LittleEndian, int32(len(f.BlockHash))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(f.BlockHash); err != nil {
		return nil, err
	}

	// Write Timestamp
	if err := binary.Write(buf, binary.LittleEndian, f.Timestamp); err != nil {
		return nil, err
	}

	// Compress using s2
	compressed := s2.Encode(nil, buf.Bytes())
	return compressed, nil
}

// DecodeFinalization decompresses and deserializes the Finalization.
func DecodeFinalization(data []byte) (*Finalization, error) {
	// Decompress using s2
	decompressed, err := s2.Decode(nil, data)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewReader(decompressed)
	finalization := &Finalization{}

	// Read BlockHash length and BlockHash
	var hashLen int32
	if err := binary.Read(buf, binary.LittleEndian, &hashLen); err != nil {
		return nil, err
	}
	hashBytes := make([]byte, hashLen)
	if _, err := buf.Read(hashBytes); err != nil {
		return nil, err
	}
	finalization.BlockHash = hashBytes

	// Read Timestamp
	if err := binary.Read(buf, binary.LittleEndian, &finalization.Timestamp); err != nil {
		return nil, err
	}

	return finalization, nil
}
