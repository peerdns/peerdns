// pkg/topology/message.go
package topology

import (
	"bytes"
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
)

// MessageType represents the type of a topology-related message.
type MessageType uint8

const (
	PeerJoinMessage       MessageType = 1 // Indicates that a new peer has joined the network
	PeerLeaveMessage      MessageType = 2 // Indicates that a peer has left the network
	TopologyUpdateMessage MessageType = 3 // Indicates a topology update message
)

// TopologyMessage represents a message related to the network topology.
type TopologyMessage struct {
	Type      MessageType // Type of the message
	SenderID  peer.ID     // ID of the peer sending the message
	PeerID    peer.ID     // ID of the peer involved in the message (e.g., joined/left)
	Addresses []string    // Addresses associated with the peer (if applicable)
}

// Serialize converts the TopologyMessage to a byte slice using raw byte buffers.
func (tm *TopologyMessage) Serialize() ([]byte, error) {
	var buffer bytes.Buffer

	// Write the MessageType (1 byte)
	if err := buffer.WriteByte(byte(tm.Type)); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write the length of SenderID (1 byte) and SenderID bytes
	senderIDBytes := []byte(tm.SenderID)
	if err := buffer.WriteByte(uint8(len(senderIDBytes))); err != nil {
		return nil, fmt.Errorf("failed to write sender ID length: %w", err)
	}
	if _, err := buffer.Write(senderIDBytes); err != nil {
		return nil, fmt.Errorf("failed to write sender ID: %w", err)
	}

	// Write the length of PeerID (1 byte) and PeerID bytes
	peerIDBytes := []byte(tm.PeerID)
	if err := buffer.WriteByte(uint8(len(peerIDBytes))); err != nil {
		return nil, fmt.Errorf("failed to write peer ID length: %w", err)
	}
	if _, err := buffer.Write(peerIDBytes); err != nil {
		return nil, fmt.Errorf("failed to write peer ID: %w", err)
	}

	// Write the number of Addresses (1 byte)
	if err := buffer.WriteByte(uint8(len(tm.Addresses))); err != nil {
		return nil, fmt.Errorf("failed to write number of addresses: %w", err)
	}

	// Write each address
	for _, address := range tm.Addresses {
		// Write the length of the address (1 byte)
		addressBytes := []byte(address)
		if err := buffer.WriteByte(uint8(len(addressBytes))); err != nil {
			return nil, fmt.Errorf("failed to write address length: %w", err)
		}
		// Write the address bytes
		if _, err := buffer.Write(addressBytes); err != nil {
			return nil, fmt.Errorf("failed to write address: %w", err)
		}
	}

	return buffer.Bytes(), nil
}

// DeserializeTopologyMessage converts a byte slice into a TopologyMessage using raw byte buffers.
func DeserializeTopologyMessage(data []byte) (*TopologyMessage, error) {
	buffer := bytes.NewBuffer(data)

	// Read the MessageType (1 byte)
	msgType, err := buffer.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	// Read the length of SenderID (1 byte) and SenderID bytes
	senderIDLength, err := buffer.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read sender ID length: %w", err)
	}
	senderIDBytes := make([]byte, senderIDLength)
	if _, err := buffer.Read(senderIDBytes); err != nil {
		return nil, fmt.Errorf("failed to read sender ID: %w", err)
	}
	senderID := peer.ID(senderIDBytes)

	// Read the length of PeerID (1 byte) and PeerID bytes
	peerIDLength, err := buffer.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read peer ID length: %w", err)
	}
	peerIDBytes := make([]byte, peerIDLength)
	if _, err := buffer.Read(peerIDBytes); err != nil {
		return nil, fmt.Errorf("failed to read peer ID: %w", err)
	}
	peerID := peer.ID(peerIDBytes)

	// Read the number of Addresses (1 byte)
	numAddresses, err := buffer.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read number of addresses: %w", err)
	}

	// Read each address
	addresses := make([]string, numAddresses)
	for i := 0; i < int(numAddresses); i++ {
		// Read the length of the address (1 byte)
		addressLength, err := buffer.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read address length: %w", err)
		}
		// Read the address bytes
		addressBytes := make([]byte, addressLength)
		if _, err := buffer.Read(addressBytes); err != nil {
			return nil, fmt.Errorf("failed to read address: %w", err)
		}
		addresses[i] = string(addressBytes)
	}

	return &TopologyMessage{
		Type:      MessageType(msgType),
		SenderID:  senderID,
		PeerID:    peerID,
		Addresses: addresses,
	}, nil
}
