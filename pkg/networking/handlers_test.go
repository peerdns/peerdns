// pkg/networking/handlers_test.go
package networking

import (
	"bytes"
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"log"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/assert"
)

func TestStreamHandler(t *testing.T) {
	// Create a new host for testing purposes.
	logger := log.New(os.Stdout, "StreamHandlerTest: ", log.LstdFlags)
	ctx := context.Background()
	h1, err := libp2p.New()
	assert.NoError(t, err, "Failed to create host 1")

	h2, err := libp2p.New()
	assert.NoError(t, err, "Failed to create host 2")

	streamHandler := NewStreamHandler(logger)

	// Set up the stream handler on host 2.
	h2.SetStreamHandler("/shponu/1.0.0", streamHandler.HandleStream)

	// Connect host 1 to host 2.
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	assert.NoError(t, err, "Failed to connect host 1 to host 2")

	// Create a new stream from host 1 to host 2.
	s, err := h1.NewStream(ctx, h2.ID(), "/shponu/1.0.0")
	assert.NoError(t, err, "Failed to create stream")

	// Send a test message.
	testMessage := NetworkMessage{
		Type:    MessageTypePing,
		From:    h1.ID().String(),
		Content: []byte("Ping!"),
	}
	data, err := SerializeMessage(testMessage)
	assert.NoError(t, err, "Failed to serialize message")

	_, err = s.Write(data)
	assert.NoError(t, err, "Failed to write message to stream")

	// Create a buffer and read the response.
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(s)
	assert.NoError(t, err, "Failed to read from stream")

	s.Close()

	// Close the hosts.
	err = h1.Close()
	assert.NoError(t, err, "Failed to close host 1")

	err = h2.Close()
	assert.NoError(t, err, "Failed to close host 2")
}
