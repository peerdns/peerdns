// pkg/topology/state.go
package topology

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Node represents a node in the topology.
type Node struct {
	PeerID    peer.ID   // Peer ID of the node
	Addresses []string  // Known addresses of the node
	Children  []peer.ID // Optional: connected child nodes for hierarchical structures
}

// NetworkTopology holds the state of the entire network.
type NetworkTopology struct {
	nodes map[peer.ID]*Node // Nodes in the network
	mutex sync.RWMutex      // Mutex for safe concurrent access
}

// NewNetworkTopology creates a new NetworkTopology instance.
func NewNetworkTopology() *NetworkTopology {
	return &NetworkTopology{
		nodes: make(map[peer.ID]*Node),
	}
}

// AddNode adds a node to the topology.
func (nt *NetworkTopology) AddNode(peerID peer.ID, addresses []string) {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()
	if _, exists := nt.nodes[peerID]; exists {
		return
	}
	nt.nodes[peerID] = &Node{PeerID: peerID, Addresses: addresses}
}

// RemoveNode removes a node from the topology.
func (nt *NetworkTopology) RemoveNode(peerID peer.ID) {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()
	delete(nt.nodes, peerID)
}

// GetNode retrieves a node by its ID.
func (nt *NetworkTopology) GetNode(peerID peer.ID) (*Node, error) {
	nt.mutex.RLock()
	defer nt.mutex.RUnlock()
	node, exists := nt.nodes[peerID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", peerID)
	}
	return node, nil
}

// GetAllNodes returns all nodes in the topology.
func (nt *NetworkTopology) GetAllNodes() []*Node {
	nt.mutex.RLock()
	defer nt.mutex.RUnlock()
	nodes := make([]*Node, 0, len(nt.nodes))
	for _, node := range nt.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}
