// pkg/networking/protocol_test.go
package networking

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerializeDeserializeMessage(t *testing.T) {
	originalMsg := NetworkMessage{
		Type:    MessageTypeRequest,
		From:    "peer1",
		Content: []byte("This is a test request."),
	}

	// Serialize the message
	serialized, err := SerializeMessage(originalMsg)
	assert.NoError(t, err, "Serialization should not return an error")

	// Deserialize the message
	deserializedMsg, err := DeserializeMessage(serialized)
	assert.NoError(t, err, "Deserialization should not return an error")

	// Compare original and deserialized messages
	assert.Equal(t, originalMsg.Type, deserializedMsg.Type, "Message types should match")
	assert.Equal(t, originalMsg.From, deserializedMsg.From, "Sender IDs should match")
	assert.Equal(t, originalMsg.Content, deserializedMsg.Content, "Message contents should match")
}

func TestDeserializeInvalidData(t *testing.T) {
	invalidData := []byte("invalid data")

	_, err := DeserializeMessage(invalidData)
	assert.Error(t, err, "Deserialization should fail for invalid data")
}
