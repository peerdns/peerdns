// pkg/state/proposal.go
package state

import (
	"bytes"
	"encoding/binary"
	"github.com/klauspost/compress/s2"
)

// Proposal represents a block proposal in the consensus process.
type Proposal struct {
	ID        string // Unique identifier for the proposal
	BlockHash []byte
	Content   []byte
	Timestamp int64 // Unix timestamp
}

// Encode serializes and compresses the Proposal.
func (p *Proposal) Encode() ([]byte, error) {
	buf := &bytes.Buffer{}

	// Write ID length and ID
	if err := binary.Write(buf, binary.LittleEndian, int32(len(p.ID))); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(p.ID); err != nil {
		return nil, err
	}

	// Write BlockHash length and BlockHash
	if err := binary.Write(buf, binary.LittleEndian, int32(len(p.BlockHash))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(p.BlockHash); err != nil {
		return nil, err
	}

	// Write Content length and Content
	if err := binary.Write(buf, binary.LittleEndian, int32(len(p.Content))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(p.Content); err != nil {
		return nil, err
	}

	// Write Timestamp
	if err := binary.Write(buf, binary.LittleEndian, p.Timestamp); err != nil {
		return nil, err
	}

	// Compress using s2
	compressed := s2.Encode(nil, buf.Bytes())
	return compressed, nil
}

// DecodeProposal decompresses and deserializes the Proposal.
func DecodeProposal(data []byte) (*Proposal, error) {
	// Decompress using s2
	decompressed, err := s2.Decode(nil, data)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewReader(decompressed)
	proposal := &Proposal{}

	// Read ID length and ID
	var idLen int32
	if err := binary.Read(buf, binary.LittleEndian, &idLen); err != nil {
		return nil, err
	}
	idBytes := make([]byte, idLen)
	if _, err := buf.Read(idBytes); err != nil {
		return nil, err
	}
	proposal.ID = string(idBytes)

	// Read BlockHash length and BlockHash
	var hashLen int32
	if err := binary.Read(buf, binary.LittleEndian, &hashLen); err != nil {
		return nil, err
	}
	hashBytes := make([]byte, hashLen)
	if _, err := buf.Read(hashBytes); err != nil {
		return nil, err
	}
	proposal.BlockHash = hashBytes

	// Read Content length and Content
	var contentLen int32
	if err := binary.Read(buf, binary.LittleEndian, &contentLen); err != nil {
		return nil, err
	}
	contentBytes := make([]byte, contentLen)
	if _, err := buf.Read(contentBytes); err != nil {
		return nil, err
	}
	proposal.Content = contentBytes

	// Read Timestamp
	if err := binary.Read(buf, binary.LittleEndian, &proposal.Timestamp); err != nil {
		return nil, err
	}

	return proposal, nil
}
