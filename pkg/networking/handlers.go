package networking

import (
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/peerdns/peerdns/pkg/packets"
)

// StreamHandler handles incoming streams and processes packets.
type StreamHandler struct {
	Logger *log.Logger
}

// NewStreamHandler creates a new StreamHandler instance.
func NewStreamHandler(logger *log.Logger) *StreamHandler {
	return &StreamHandler{Logger: logger}
}

// HandleStream processes incoming streams and packets.
func (sh *StreamHandler) HandleStream(s network.Stream) {
	defer s.Close()

	peerID := s.Conn().RemotePeer()
	sh.Logger.Printf("New stream from peer: %s", peerID)

	// Read the incoming data.
	buf := make([]byte, 4096)
	n, err := s.Read(buf)
	if err != nil {
		sh.Logger.Printf("Error reading from stream: %v", err)
		return
	}

	// Deserialize the packet.
	packet, err := packets.DeserializeNetworkPacket(buf[:n])
	if err != nil {
		sh.Logger.Printf("Failed to deserialize packet: %v", err)
		return
	}

	// Process the packet based on its type.
	switch packet.Type {
	case packets.PacketTypePing:
		sh.Logger.Printf("Received Ping from %s", packet.SenderID)
		// Optionally, send a Pong response here.
	case packets.PacketTypeRequest:
		sh.Logger.Printf("Received Request from %s", packet.SenderID)
		// Handle the request and potentially send a response.
	case packets.PacketTypeResponse:
		sh.Logger.Printf("Received Response from %s", packet.SenderID)
		// Handle the response.
	default:
		sh.Logger.Printf("Unknown packet type from %s", packet.SenderID)
	}
}

// SendResponse sends a response packet to a specific peer using the stream.
func (sh *StreamHandler) SendResponse(s network.Stream, response packets.NetworkPacket) error {
	// Set the packet type to Response.
	response.Type = packets.PacketTypeResponse

	// Serialize the response packet.
	data, err := response.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	// Write the serialized data to the stream.
	_, err = s.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}
