// pkg/networking/handlers.go
package networking

import (
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/network"
)

// StreamHandler handles incoming streams and processes messages.
type StreamHandler struct {
	Logger *log.Logger
}

// NewStreamHandler creates a new StreamHandler instance.
func NewStreamHandler(logger *log.Logger) *StreamHandler {
	return &StreamHandler{Logger: logger}
}

// HandleStream processes incoming streams and messages.
func (sh *StreamHandler) HandleStream(s network.Stream) {
	defer s.Close()

	peerID := s.Conn().RemotePeer()
	sh.Logger.Printf("New stream from peer: %s", peerID)

	// Read the incoming message.
	buf := make([]byte, 4096)
	n, err := s.Read(buf)
	if err != nil {
		sh.Logger.Printf("Error reading from stream: %v", err)
		return
	}

	// Deserialize the message.
	msg, err := DeserializeMessage(buf[:n])
	if err != nil {
		sh.Logger.Printf("Failed to deserialize message: %v", err)
		return
	}

	// Process the message based on its type.
	switch msg.Type {
	case MessageTypePing:
		sh.Logger.Printf("Received Ping from %s", msg.From)
	case MessageTypeRequest:
		sh.Logger.Printf("Received Request from %s", msg.From)
	case MessageTypeResponse:
		sh.Logger.Printf("Received Response from %s", msg.From)
	default:
		sh.Logger.Printf("Unknown message type from %s", msg.From)
	}
}

// SendResponse sends a response message to a specific peer using the stream.
func (sh *StreamHandler) SendResponse(s network.Stream, response NetworkMessage) error {
	data, err := SerializeMessage(response)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	_, err = s.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}
