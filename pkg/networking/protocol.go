package networking

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// MessageType represents the type of messages exchanged in the network.
type MessageType int

const (
	// MessageTypeUnknown represents an unknown message type.
	MessageTypeUnknown MessageType = iota

	// MessageTypePing represents a simple ping message.
	MessageTypePing

	// MessageTypeRequest represents a request message.
	MessageTypeRequest

	// MessageTypeResponse represents a response message.
	MessageTypeResponse
)

// NetworkMessage represents the structure of messages exchanged in the network.
type NetworkMessage struct {
	Type    MessageType // Type of the message
	From    string      // Sender's peer ID as a string
	Content []byte      // Serialized content of the message
}

// SerializeMessage encodes a NetworkMessage into bytes.
func SerializeMessage(msg NetworkMessage) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	return buf.Bytes(), nil
}

// DeserializeMessage decodes bytes into a NetworkMessage.
func DeserializeMessage(data []byte) (NetworkMessage, error) {
	var msg NetworkMessage
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&msg); err != nil {
		return msg, fmt.Errorf("failed to decode message: %w", err)
	}
	return msg, nil
}
