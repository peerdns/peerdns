// pkg/consensus/message.go
package consensus

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

// MessageType represents the type of a consensus-related message.
type MessageType uint8

const (
	ProposalMessage     MessageType = 1 // Indicates a new block proposal
	ApprovalMessage     MessageType = 2 // Indicates approval of a proposal
	FinalizationMessage MessageType = 3 // Indicates block finalization
)

// ConsensusMessage represents a message exchanged during the consensus protocol.
type ConsensusMessage struct {
	Type        MessageType   // Type of the message (Proposal, Approval, Finalization)
	ProposerID  peer.ID       // ID of the validator proposing the block (for ProposalMessage)
	ValidatorID peer.ID       // ID of the validator approving the proposal (for ApprovalMessage)
	BlockHash   []byte        // Hash of the block involved in the message
	BlockData   []byte        // Raw block data (optional, used in ProposalMessage)
	Signature   *BLSSignature // BLS signature for the message
}

// Serialize serializes a ConsensusMessage into a byte slice.
func (cm *ConsensusMessage) Serialize() ([]byte, error) {
	var buffer bytes.Buffer

	// Serialize the MessageType
	if err := binary.Write(&buffer, binary.LittleEndian, cm.Type); err != nil {
		return nil, fmt.Errorf("failed to serialize message type: %w", err)
	}

	// Serialize ProposerID and ValidatorID
	if err := serializePeerID(&buffer, cm.ProposerID); err != nil {
		return nil, fmt.Errorf("failed to serialize proposer ID: %w", err)
	}
	if err := serializePeerID(&buffer, cm.ValidatorID); err != nil {
		return nil, fmt.Errorf("failed to serialize validator ID: %w", err)
	}

	// Serialize BlockHash length and BlockHash
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cm.BlockHash))); err != nil {
		return nil, fmt.Errorf("failed to serialize block hash length: %w", err)
	}
	if _, err := buffer.Write(cm.BlockHash); err != nil {
		return nil, fmt.Errorf("failed to serialize block hash: %w", err)
	}

	// Serialize BlockData length and BlockData (if applicable)
	if cm.Type == ProposalMessage {
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cm.BlockData))); err != nil {
			return nil, fmt.Errorf("failed to serialize block data length: %w", err)
		}
		if _, err := buffer.Write(cm.BlockData); err != nil {
			return nil, fmt.Errorf("failed to serialize block data: %w", err)
		}
	}

	// Serialize Signature length and Signature
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cm.Signature.Signature))); err != nil {
		return nil, fmt.Errorf("failed to serialize signature length: %w", err)
	}
	if _, err := buffer.Write(cm.Signature.Signature); err != nil {
		return nil, fmt.Errorf("failed to serialize signature: %w", err)
	}

	return buffer.Bytes(), nil
}

// DeserializeConsensusMessage deserializes a byte slice into a ConsensusMessage.
func DeserializeConsensusMessage(data []byte) (*ConsensusMessage, error) {
	buffer := bytes.NewBuffer(data)
	cm := &ConsensusMessage{}

	// Deserialize MessageType
	if err := binary.Read(buffer, binary.LittleEndian, &cm.Type); err != nil {
		return nil, fmt.Errorf("failed to deserialize message type: %w", err)
	}

	// Deserialize ProposerID and ValidatorID
	proposerID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize proposer ID: %w", err)
	}
	cm.ProposerID = proposerID

	validatorID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize validator ID: %w", err)
	}
	cm.ValidatorID = validatorID

	// Deserialize BlockHash length and BlockHash
	var blockHashLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &blockHashLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize block hash length: %w", err)
	}
	cm.BlockHash = make([]byte, blockHashLen)
	if _, err := buffer.Read(cm.BlockHash); err != nil {
		return nil, fmt.Errorf("failed to deserialize block hash: %w", err)
	}

	// Deserialize BlockData length and BlockData (if applicable)
	if cm.Type == ProposalMessage {
		var blockDataLen uint32
		if err := binary.Read(buffer, binary.LittleEndian, &blockDataLen); err != nil {
			return nil, fmt.Errorf("failed to deserialize block data length: %w", err)
		}
		cm.BlockData = make([]byte, blockDataLen)
		if _, err := buffer.Read(cm.BlockData); err != nil {
			return nil, fmt.Errorf("failed to deserialize block data: %w", err)
		}
	}

	// Deserialize Signature length and Signature
	var signatureLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &signatureLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize signature length: %w", err)
	}
	signature := make([]byte, signatureLen)
	if _, err := buffer.Read(signature); err != nil {
		return nil, fmt.Errorf("failed to deserialize signature: %w", err)
	}
	cm.Signature = &BLSSignature{Signature: signature}

	return cm, nil
}

// serializePeerID serializes a peer ID into a byte slice and writes it to the buffer.
func serializePeerID(buffer *bytes.Buffer, id peer.ID) error {
	peerIDBytes := []byte(id)
	if err := binary.Write(buffer, binary.LittleEndian, uint32(len(peerIDBytes))); err != nil {
		return fmt.Errorf("failed to serialize peer ID length: %w", err)
	}
	if _, err := buffer.Write(peerIDBytes); err != nil {
		return fmt.Errorf("failed to serialize peer ID: %w", err)
	}
	return nil
}

// deserializePeerID deserializes a peer ID from the buffer.
func deserializePeerID(buffer *bytes.Buffer) (peer.ID, error) {
	var peerIDLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &peerIDLen); err != nil {
		return "", fmt.Errorf("failed to deserialize peer ID length: %w", err)
	}
	peerIDBytes := make([]byte, peerIDLen)
	if _, err := buffer.Read(peerIDBytes); err != nil {
		return "", fmt.Errorf("failed to deserialize peer ID: %w", err)
	}
	return peer.ID(peerIDBytes), nil
}
