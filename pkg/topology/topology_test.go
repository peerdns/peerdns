// pkg/topology/topology_test.go
package topology

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

// Setup function for testing.
func setupTopologyTest() (*Topology, context.Context) {
	logger := log.New(os.Stdout, "TopologyTest: ", log.LstdFlags)
	ctx := context.Background()
	topology := NewTopology(ctx, logger)
	return topology, ctx
}

// TestAddAndRemovePeer tests the addition and removal of peers.
func TestAddAndRemovePeer(t *testing.T) {
	topology, _ := setupTopologyTest()

	peerID := peer.ID("test-peer-1")
	addresses := []string{"/ip4/127.0.0.1/tcp/9000"}

	// Test adding a peer.
	err := topology.AddPeer(peerID, addresses)
	assert.NoError(t, err, "Failed to add peer")

	// Verify the peer exists.
	node, err := topology.state.GetNode(peerID)
	assert.NoError(t, err, "Failed to get node")
	assert.Equal(t, peerID, node.PeerID, "Peer ID mismatch")
	assert.Equal(t, addresses, node.Addresses, "Address mismatch")

	// Test removing the peer.
	err = topology.RemovePeer(peerID)
	assert.NoError(t, err, "Failed to remove peer")

	// Verify the peer no longer exists.
	_, err = topology.state.GetNode(peerID)
	assert.Error(t, err, "Node should not exist after removal")
}

// TestMessageSerialization tests serialization and deserialization.
func TestMessageSerialization(t *testing.T) {
	originalMessage := &TopologyMessage{
		Type:      PeerJoinMessage,
		SenderID:  peer.ID("sender"),
		PeerID:    peer.ID("joined-peer"),
		Addresses: []string{"/ip4/127.0.0.1/tcp/9000"},
	}

	// Serialize the message.
	serialized, err := originalMessage.Serialize()
	assert.NoError(t, err, "Failed to serialize")

	// Deserialize the message.
	deserialized, err := DeserializeTopologyMessage(serialized)
	assert.NoError(t, err, "Failed to deserialize")

	// Verify equality.
	assert.Equal(t, originalMessage.Type, deserialized.Type, "Message Type mismatch")
	assert.Equal(t, originalMessage.SenderID, deserialized.SenderID, "SenderID mismatch")
	assert.Equal(t, originalMessage.PeerID, deserialized.PeerID, "PeerID mismatch")
	assert.Equal(t, originalMessage.Addresses, deserialized.Addresses, "Addresses mismatch")
}
