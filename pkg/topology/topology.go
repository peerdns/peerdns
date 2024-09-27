// pkg/topology/topology.go
package topology

import (
	"context"
	"log"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Topology orchestrates the overall network topology and peer management.
type Topology struct {
	manager *TopologyManager // The topology manager for peer management
	state   *NetworkTopology // The state management for the network topology
	ctx     context.Context  // Context for lifecycle management
	logger  *log.Logger      // Logger for topology activities
}

// NewTopology creates a new Topology instance with the given logger.
func NewTopology(ctx context.Context, logger *log.Logger) *Topology {
	// Create the shared NetworkTopology state
	nt := NewNetworkTopology()

	// Create the TopologyManager with a reference to the shared state
	tm := NewTopologyManager(ctx, logger, nt)

	return &Topology{
		manager: tm,
		state:   nt,
		ctx:     ctx,
		logger:  logger,
	}
}

// AddPeer adds a new peer to the topology using the topology manager.
func (t *Topology) AddPeer(peerID peer.ID, addresses []string) error {
	return t.manager.AddPeer(peerID, addresses)
}

// RemovePeer removes a peer from the topology.
func (t *Topology) RemovePeer(peerID peer.ID) error {
	return t.manager.RemovePeer(peerID)
}

// GetNode retrieves a node from the network state.
func (t *Topology) GetNode(peerID peer.ID) (*Node, error) {
	return t.state.GetNode(peerID)
}

// BroadcastTopologyMessage broadcasts a topology update message to all peers.
func (t *Topology) BroadcastTopologyMessage(msg *TopologyMessage) error {
	_, err := msg.Serialize()
	if err != nil {
		return err
	}

	// Simulate message broadcasting (replace with actual networking code).
	for _, peer := range t.manager.GetPeers() {
		t.logger.Printf("Broadcasting message to peer: %s", peer.ID)
	}
	return nil
}
