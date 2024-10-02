// pkg/networking/p2p.go
package networking

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/privacy"
	"sync"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// Message represents a message received from the network.
type Message struct {
	Data []byte
}

// P2PNetwork defines the core structure of the P2P network.
type P2PNetwork struct {
	Host           host.Host
	PubSub         *pubsub.PubSub
	Topic          *pubsub.Topic
	ProtocolID     protocol.ID
	Ctx            context.Context
	Cancel         context.CancelFunc
	Peers          map[peer.ID]*PeerInfo
	cfg            config.Networking // Store the configuration in the P2PNetwork struct
	mu             sync.RWMutex
	Logger         logger.Logger
	PrivacyManager *privacy.PrivacyManager
	mdns           mdns.Service
}

// PeerInfo holds information about connected peers.
type PeerInfo struct {
	ID        peer.ID
	Addresses []multiaddr.Multiaddr
	Metadata  map[string]string
}

// NewP2PNetwork initializes and returns a new P2P network with the given parameters.
func NewP2PNetwork(ctx context.Context, cfg config.Networking, logger logger.Logger) (*P2PNetwork, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Generate a new RSA key pair for the P2P host.
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create a new libp2p host with configuration from cfg.
	host, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.ListenPort)),
		libp2p.Identity(priv),
		libp2p.NATPortMap(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Initialize PubSub using GossipSub.
	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	// Join or create a topic using ProtocolID from config.
	topic, err := ps.Join(cfg.ProtocolID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	network := &P2PNetwork{
		Host:       host,
		PubSub:     ps,
		Topic:      topic,
		ProtocolID: protocol.ID(cfg.ProtocolID), // Use ProtocolID from config
		Ctx:        ctx,
		Cancel:     cancel,
		Peers:      make(map[peer.ID]*PeerInfo),
		cfg:        cfg, // Store the config in the struct
		Logger:     logger,
	}

	if cfg.EnableMDNS {
		// Set up mDNS discovery
		mdnsService := mdns.NewMdnsService(host, "peerdns-mdns", &mdnsNotifee{n: network})
		err := mdnsService.Start()
		if err != nil {
			logger.Error("Failed to start mDNS service", zap.Error(err))
		} else {
			network.mdns = mdnsService
			logger.Info("mDNS service started")
		}
	}

	// Set the stream handler for the custom protocol.
	host.SetStreamHandler(network.ProtocolID, network.handleStream)

	network.Logger.Info("P2P network initialized",
		zap.String("protocolID", cfg.ProtocolID),
		zap.Int("listenPort", cfg.ListenPort),
		zap.String("hostID", host.ID().String()),
	)

	// Connect to bootstrap peers if any
	for _, peerAddr := range cfg.BootstrapPeers {
		err := network.ConnectPeer(peerAddr)
		if err != nil {
			logger.Warn("Failed to connect to bootstrap peer", zap.String("peer", peerAddr), zap.Error(err))
		} else {
			logger.Info("Connected to bootstrap peer", zap.String("peer", peerAddr))
		}
	}

	return network, nil
}

// ConnectPeer connects to a given peer using its multiaddress.
func (n *P2PNetwork) ConnectPeer(peerAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	n.Host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

	if err := n.Host.Connect(n.Ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", peerInfo.ID, err)
	}

	n.addPeer(peerInfo.ID, peerInfo.Addrs)
	n.Logger.Info("Connected to peer",
		zap.String("peerID", peerInfo.ID.String()),
		zap.Strings("addresses", addrStrings(peerInfo.Addrs)),
	)

	return nil
}

// SendMessage sends a direct message to a specific peer using the custom protocol.
func (n *P2PNetwork) SendMessage(target peer.ID, message []byte) error {
	stream, err := n.Host.NewStream(n.Ctx, target, n.ProtocolID)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	_, err = stream.Write(message)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	n.Logger.Info("Sent message",
		zap.String("toPeer", target.String()),
		zap.ByteString("message", message),
	)

	return nil
}

// BroadcastMessage sends a message to all peers subscribed to the PubSub topic.
func (n *P2PNetwork) BroadcastMessage(message []byte) error {
	err := n.Topic.Publish(n.Ctx, message)
	if err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	n.Logger.Info("Broadcasted message",
		zap.ByteString("message", message),
	)

	return nil
}

// handleStream processes incoming streams using the custom protocol.
func (n *P2PNetwork) handleStream(s network.Stream) {
	defer s.Close()

	peerID := s.Conn().RemotePeer()
	n.Logger.Info("Received stream from peer",
		zap.String("peerID", peerID.String()),
	)

	// Read the incoming message.
	buf := make([]byte, 4096)
	numBytes, err := s.Read(buf)
	if err != nil {
		n.Logger.Error("Error reading from stream",
			zap.Error(err),
			zap.String("peerID", peerID.String()),
		)
		return
	}

	message := buf[:numBytes]

	n.Logger.Info("Received message",
		zap.String("fromPeer", peerID.String()),
		zap.ByteString("message", message),
	)
}

// Shutdown gracefully shuts down the P2P network.
func (n *P2PNetwork) Shutdown() {
	n.Logger.Info("Shutting down P2P network")
	n.Cancel()

	if n.mdns != nil {
		if err := n.mdns.Close(); err != nil {
			n.Logger.Error("Error closing mDNS service", zap.Error(err))
		} else {
			n.Logger.Info("mDNS service closed successfully")
		}
	}

	if err := n.Host.Close(); err != nil {
		n.Logger.Error("Error closing host", zap.Error(err))
	} else {
		n.Logger.Info("Host closed successfully")
	}
}

// addPeer adds a peer to the internal peer list.
func (n *P2PNetwork) addPeer(id peer.ID, addrs []multiaddr.Multiaddr) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.Peers[id]; !exists {
		n.Peers[id] = &PeerInfo{
			ID:        id,
			Addresses: addrs,
			Metadata:  make(map[string]string),
		}
		n.Logger.Info("Peer added",
			zap.String("peerID", id.String()),
			zap.Strings("addresses", addrStrings(addrs)),
		)
	}
}

// RemovePeer removes a peer from the internal peer list.
func (n *P2PNetwork) RemovePeer(id peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.Peers[id]; exists {
		delete(n.Peers, id)
		n.Logger.Info("Peer removed",
			zap.String("peerID", id.String()),
		)
	}
}

// GetPeers returns a list of currently connected peers.
func (n *P2PNetwork) GetPeers() []*PeerInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(n.Peers))
	for _, peerInfo := range n.Peers {
		peers = append(peers, peerInfo)
	}
	return peers
}

// addrStrings converts a slice of Multiaddrs to a slice of strings.
func addrStrings(addrs []multiaddr.Multiaddr) []string {
	strs := make([]string, len(addrs))
	for i, addr := range addrs {
		strs[i] = addr.String()
	}
	return strs
}
