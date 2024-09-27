// pkg/topology/manager.go
package topology

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TopologyManager manages the network topology and peer connections.
type TopologyManager struct {
	peers    map[peer.ID]*PeerInfo // Map of connected peers
	mutex    sync.RWMutex          // Mutex for synchronizing access to peers
	logger   *log.Logger           // Logger for topology events
	ctx      context.Context       // Context for managing lifecycle
	cancel   context.CancelFunc    // Cancel function to stop the manager
	topology *NetworkTopology      // Reference to the network topology state
}

// PeerInfo represents information about a connected peer.
type PeerInfo struct {
	ID        peer.ID  // Peer ID
	Addresses []string // List of known addresses for the peer
	Status    string   // Connection status (e.g., "connected", "disconnected")
}

// NewTopologyManager creates a new topology manager with the provided context and logger.
func NewTopologyManager(ctx context.Context, logger *log.Logger, topology *NetworkTopology) *TopologyManager {
	// Create a child context and cancel function for the manager.
	childCtx, cancel := context.WithCancel(ctx)

	return &TopologyManager{
		peers:    make(map[peer.ID]*PeerInfo),
		logger:   logger,
		ctx:      childCtx,
		cancel:   cancel,
		topology: topology, // Reference to the shared network topology
	}
}

// AddPeer adds a new peer to the network topology.
func (tm *TopologyManager) AddPeer(peerID peer.ID, addresses []string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Check if the peer already exists in the topology.
	if _, exists := tm.peers[peerID]; exists {
		return fmt.Errorf("peer %s already exists in topology", peerID)
	}

	// Create a new PeerInfo entry and add it to the map.
	peerInfo := &PeerInfo{
		ID:        peerID,
		Addresses: addresses,
		Status:    "connected",
	}
	tm.peers[peerID] = peerInfo

	// Update the network topology state.
	tm.topology.AddNode(peerID, addresses)

	tm.logger.Printf("Added peer %s to topology", peerID)
	return nil
}

// RemovePeer removes a peer from the network topology.
func (tm *TopologyManager) RemovePeer(peerID peer.ID) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Check if the peer exists in the topology.
	if _, exists := tm.peers[peerID]; !exists {
		return fmt.Errorf("peer %s not found in topology", peerID)
	}

	// Remove the peer from the map.
	delete(tm.peers, peerID)

	// Update the network topology state.
	tm.topology.RemoveNode(peerID)

	tm.logger.Printf("Removed peer %s from topology", peerID)
	return nil
}

// GetPeers returns a list of currently connected peers.
func (tm *TopologyManager) GetPeers() []*PeerInfo {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	// Collect all connected peers.
	peers := make([]*PeerInfo, 0, len(tm.peers))
	for _, peer := range tm.peers {
		peers = append(peers, peer)
	}
	return peers
}

// Shutdown gracefully shuts down the topology manager.
func (tm *TopologyManager) Shutdown() {
	tm.cancel()
	tm.logger.Println("Topology manager shut down successfully")
}
