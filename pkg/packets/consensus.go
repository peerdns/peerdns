package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/encryption"
)

// ConsensusPacket represents a packet exchanged during the consensus protocol.
type ConsensusPacket struct {
	Type        PacketType               // Type of the packet (Proposal, Approval, Finalization)
	ProposerID  peer.ID                  // ID of the validator proposing the block (for ProposalPacket)
	ValidatorID peer.ID                  // ID of the validator approving the proposal (for ApprovalPacket)
	BlockHash   []byte                   // Hash of the block involved in the packet
	BlockData   []byte                   // Raw block data (optional, used in ProposalPacket)
	Signature   *encryption.BLSSignature // BLS signature for the packet
}

// Serialize serializes a ConsensusPacket into a byte slice.
// It handles nil Signature gracefully.
func (cp *ConsensusPacket) Serialize() ([]byte, error) {
	var buffer bytes.Buffer

	// Serialize the PacketType
	if err := binary.Write(&buffer, binary.LittleEndian, cp.Type); err != nil {
		return nil, fmt.Errorf("failed to serialize packet type: %w", err)
	}

	// Serialize ProposerID and ValidatorID
	if err := serializePeerID(&buffer, cp.ProposerID); err != nil {
		return nil, fmt.Errorf("failed to serialize proposer ID: %w", err)
	}
	if err := serializePeerID(&buffer, cp.ValidatorID); err != nil {
		return nil, fmt.Errorf("failed to serialize validator ID: %w", err)
	}

	// Serialize BlockHash length and BlockHash
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cp.BlockHash))); err != nil {
		return nil, fmt.Errorf("failed to serialize block hash length: %w", err)
	}
	if _, err := buffer.Write(cp.BlockHash); err != nil {
		return nil, fmt.Errorf("failed to serialize block hash: %w", err)
	}

	// Serialize BlockData length and BlockData (if applicable)
	if cp.Type == PacketTypeProposal {
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cp.BlockData))); err != nil {
			return nil, fmt.Errorf("failed to serialize block data length: %w", err)
		}
		if _, err := buffer.Write(cp.BlockData); err != nil {
			return nil, fmt.Errorf("failed to serialize block data: %w", err)
		}
	} else {
		// For non-ProposalPacket types, serialize block data length as 0
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize empty block data length: %w", err)
		}
	}

	// Serialize Signature presence flag and Signature
	if cp.Signature != nil && len(cp.Signature.Signature) > 0 {
		// Indicate that a signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(1)); err != nil {
			return nil, fmt.Errorf("failed to serialize signature presence flag: %w", err)
		}
		// Serialize Signature length and Signature
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cp.Signature.Signature))); err != nil {
			return nil, fmt.Errorf("failed to serialize signature length: %w", err)
		}
		if _, err := buffer.Write(cp.Signature.Signature); err != nil {
			return nil, fmt.Errorf("failed to serialize signature: %w", err)
		}
	} else {
		// Indicate that no signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize signature absence flag: %w", err)
		}
		// Serialize Signature length as 0
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize empty signature length: %w", err)
		}
	}

	return buffer.Bytes(), nil
}

// DeserializeConsensusPacket deserializes a byte slice into a ConsensusPacket.
func DeserializeConsensusPacket(data []byte) (*ConsensusPacket, error) {
	buffer := bytes.NewBuffer(data)
	cp := &ConsensusPacket{}

	// Deserialize PacketType
	if err := binary.Read(buffer, binary.LittleEndian, &cp.Type); err != nil {
		return nil, fmt.Errorf("failed to deserialize packet type: %w", err)
	}

	// Deserialize ProposerID and ValidatorID
	proposerID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize proposer ID: %w", err)
	}
	cp.ProposerID = proposerID

	validatorID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize validator ID: %w", err)
	}
	cp.ValidatorID = validatorID

	// Deserialize BlockHash length and BlockHash
	var blockHashLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &blockHashLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize block hash length: %w", err)
	}
	cp.BlockHash = make([]byte, blockHashLen)
	if _, err := buffer.Read(cp.BlockHash); err != nil {
		return nil, fmt.Errorf("failed to deserialize block hash: %w", err)
	}

	// Deserialize BlockData length and BlockData (if applicable)
	if cp.Type == PacketTypeProposal {
		var blockDataLen uint32
		if err := binary.Read(buffer, binary.LittleEndian, &blockDataLen); err != nil {
			return nil, fmt.Errorf("failed to deserialize block data length: %w", err)
		}
		if blockDataLen > 0 {
			cp.BlockData = make([]byte, blockDataLen)
			if _, err := buffer.Read(cp.BlockData); err != nil {
				return nil, fmt.Errorf("failed to deserialize block data: %w", err)
			}
		}
	} else {
		// For non-ProposalPacket types, read and discard the block data
		var blockDataLen uint32
		if err := binary.Read(buffer, binary.LittleEndian, &blockDataLen); err != nil {
			return nil, fmt.Errorf("failed to deserialize empty block data length: %w", err)
		}
		if blockDataLen > 0 {
			discard := make([]byte, blockDataLen)
			if _, err := buffer.Read(discard); err != nil {
				return nil, fmt.Errorf("failed to discard block data: %w", err)
			}
		}
	}

	// Deserialize Signature presence flag and Signature
	var sigPresence uint8
	if err := binary.Read(buffer, binary.LittleEndian, &sigPresence); err != nil {
		return nil, fmt.Errorf("failed to deserialize signature presence flag: %w", err)
	}

	var sigLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &sigLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize signature length: %w", err)
	}

	if sigPresence == 1 && sigLen > 0 {
		signature := make([]byte, sigLen)
		if _, err := buffer.Read(signature); err != nil {
			return nil, fmt.Errorf("failed to deserialize signature: %w", err)
		}
		cp.Signature = &encryption.BLSSignature{Signature: signature}
	} else {
		cp.Signature = nil
	}

	return cp, nil
}
