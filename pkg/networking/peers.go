package networking

import (
	"github.com/peerdns/peerdns/pkg/logger"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// PeerInfo holds information about connected peers.
type PeerInfo struct {
	ID        peer.ID
	Addresses []multiaddr.Multiaddr
	Metadata  map[string]string
}

// PeersManager manages the collection of peers in the network.
type PeersManager struct {
	peers  map[peer.ID]*PeerInfo
	mu     sync.RWMutex
	Logger logger.Logger
}

// NewPeersManager initializes and returns a new PeersManager.
func NewPeersManager(logger logger.Logger) *PeersManager {
	return &PeersManager{
		peers:  make(map[peer.ID]*PeerInfo),
		Logger: logger,
	}
}

// AddPeer adds a new peer to the manager.
// If the peer already exists, it updates the addresses.
func (pm *PeersManager) AddPeer(id peer.ID, addrs []multiaddr.Multiaddr) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if existingPeer, exists := pm.peers[id]; exists {
		existingPeer.Addresses = addrs
		pm.Logger.Info("Updated peer addresses",
			zap.String("peerID", id.String()),
			zap.Strings("addresses", multiaddrsToStrings(addrs)),
		)
	} else {
		pm.peers[id] = &PeerInfo{
			ID:        id,
			Addresses: addrs,
			Metadata:  make(map[string]string),
		}
		pm.Logger.Info("Added new peer",
			zap.String("peerID", id.String()),
			zap.Strings("addresses", multiaddrsToStrings(addrs)),
		)
	}
}

// RemovePeer removes a peer from the manager.
func (pm *PeersManager) RemovePeer(id peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.peers[id]; exists {
		delete(pm.peers, id)
		pm.Logger.Info("Removed peer",
			zap.String("peerID", id.String()),
		)
	}
}

// GetPeers retrieves a snapshot of all current peers.
func (pm *PeersManager) GetPeers() []*PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(pm.peers))
	for _, peerInfo := range pm.peers {
		peers = append(peers, peerInfo)
	}
	return peers
}

// multiaddrsToStrings converts a slice of Multiaddrs to a slice of strings.
func multiaddrsToStrings(addrs []multiaddr.Multiaddr) []string {
	strs := make([]string, len(addrs))
	for i, addr := range addrs {
		strs[i] = addr.String()
	}
	return strs
}
