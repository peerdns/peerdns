package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/encryption"
)

// NetworkPacket represents the structure of packets exchanged in the network.
type NetworkPacket struct {
	Type       PacketType               // Type of the packet (Ping, Request, Response)
	SenderID   peer.ID                  // ID of the sender
	ReceiverID peer.ID                  // ID of the receiver (optional, can be zero value for broadcasts)
	Payload    []byte                   // Payload of the packet
	Signature  *encryption.BLSSignature // BLS signature for the packet (optional)
}

// Serialize serializes the NetworkPacket into a byte slice.
// It handles optional fields gracefully.
func (np *NetworkPacket) Serialize() ([]byte, error) {
	var buffer bytes.Buffer

	// Serialize PacketType
	if err := binary.Write(&buffer, binary.LittleEndian, np.Type); err != nil {
		return nil, fmt.Errorf("failed to serialize packet type: %w", err)
	}

	// Serialize SenderID
	if err := serializePeerID(&buffer, np.SenderID); err != nil {
		return nil, fmt.Errorf("failed to serialize sender ID: %w", err)
	}

	// Serialize ReceiverID
	if err := serializePeerID(&buffer, np.ReceiverID); err != nil {
		return nil, fmt.Errorf("failed to serialize receiver ID: %w", err)
	}

	// Serialize Payload length and Payload
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(np.Payload))); err != nil {
		return nil, fmt.Errorf("failed to serialize payload length: %w", err)
	}
	if _, err := buffer.Write(np.Payload); err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	// Serialize Signature presence flag and Signature
	if np.Signature != nil && len(np.Signature.Signature) > 0 {
		// Indicate that a signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(1)); err != nil {
			return nil, fmt.Errorf("failed to serialize signature presence flag: %w", err)
		}
		// Serialize Signature length and Signature
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(np.Signature.Signature))); err != nil {
			return nil, fmt.Errorf("failed to serialize signature length: %w", err)
		}
		if _, err := buffer.Write(np.Signature.Signature); err != nil {
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

// DeserializeNetworkPacket deserializes a byte slice into a NetworkPacket.
// It handles optional fields gracefully.
func DeserializeNetworkPacket(data []byte) (*NetworkPacket, error) {
	buffer := bytes.NewReader(data)
	np := &NetworkPacket{}

	// Deserialize PacketType
	if err := binary.Read(buffer, binary.LittleEndian, &np.Type); err != nil {
		return nil, fmt.Errorf("failed to deserialize packet type: %w", err)
	}

	// Deserialize SenderID
	senderID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize sender ID: %w", err)
	}
	np.SenderID = senderID

	// Deserialize ReceiverID
	receiverID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize receiver ID: %w", err)
	}
	np.ReceiverID = receiverID

	// Deserialize Payload length and Payload
	var payloadLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &payloadLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize payload length: %w", err)
	}
	np.Payload = make([]byte, payloadLen)
	if _, err := buffer.Read(np.Payload); err != nil {
		return nil, fmt.Errorf("failed to deserialize payload: %w", err)
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
		np.Signature = &encryption.BLSSignature{Signature: signature}
	} else {
		np.Signature = nil
	}

	return np, nil
}
