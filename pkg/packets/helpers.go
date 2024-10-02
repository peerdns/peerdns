package packets

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/peer"
)

// serializePeerID serializes a peer ID into the writer.
func serializePeerID(writer io.Writer, id peer.ID) error {
	peerIDBytes := []byte(id)
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(peerIDBytes))); err != nil {
		return fmt.Errorf("failed to serialize peer ID length: %w", err)
	}
	if _, err := writer.Write(peerIDBytes); err != nil {
		return fmt.Errorf("failed to serialize peer ID: %w", err)
	}
	return nil
}

// deserializePeerID deserializes a peer ID from the reader.
func deserializePeerID(reader io.Reader) (peer.ID, error) {
	var peerIDLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &peerIDLen); err != nil {
		return "", fmt.Errorf("failed to deserialize peer ID length: %w", err)
	}
	peerIDBytes := make([]byte, peerIDLen)
	if _, err := io.ReadFull(reader, peerIDBytes); err != nil {
		return "", fmt.Errorf("failed to deserialize peer ID: %w", err)
	}
	return peer.ID(peerIDBytes), nil
}
