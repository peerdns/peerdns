package networking

import (
	"context"
	"fmt"
	"log"
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
)

// Message represents a message received from the network.
type Message struct {
	Data []byte
}

// P2PNetwork defines the core structure of the P2P network.
type P2PNetwork struct {
	Host       host.Host
	PubSub     *pubsub.PubSub
	Topic      *pubsub.Topic
	ProtocolID protocol.ID
	Ctx        context.Context
	Cancel     context.CancelFunc
	Peers      map[peer.ID]*PeerInfo
	mu         sync.RWMutex
	Logger     *log.Logger
}

// PeerInfo holds information about connected peers.
type PeerInfo struct {
	ID        peer.ID
	Addresses []multiaddr.Multiaddr
	Metadata  map[string]string
}

// NewP2PNetwork initializes and returns a new P2P network with the given parameters.
func NewP2PNetwork(ctx context.Context, listenPort int, protocolID string, logger *log.Logger) (*P2PNetwork, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Generate a new RSA key pair for the P2P host.
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create a new libp2p host with default options.
	host, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
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

	// Join or create a topic.
	topic, err := ps.Join(protocolID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	network := &P2PNetwork{
		Host:       host,
		PubSub:     ps,
		Topic:      topic,
		ProtocolID: protocol.ID(protocolID),
		Ctx:        ctx,
		Cancel:     cancel,
		Peers:      make(map[peer.ID]*PeerInfo),
		Logger:     logger,
	}

	// Set the stream handler for the custom protocol.
	host.SetStreamHandler(network.ProtocolID, network.handleStream)

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
	n.Logger.Printf("Connected to peer: %s", peerInfo.ID)

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

	n.Logger.Printf("Sent message to %s: %s", target, string(message))
	return nil
}

// BroadcastMessage sends a message to all peers subscribed to the PubSub topic.
func (n *P2PNetwork) BroadcastMessage(message []byte) error {
	return n.Topic.Publish(n.Ctx, message)
}

// handleStream processes incoming streams using the custom protocol.
func (n *P2PNetwork) handleStream(s network.Stream) {
	defer s.Close()

	peerID := s.Conn().RemotePeer()
	n.Logger.Printf("Received stream from peer: %s", peerID)

	// Read the incoming message.
	buf := make([]byte, 4096)
	numBytes, err := s.Read(buf)
	if err != nil {
		n.Logger.Printf("Error reading from stream: %v", err)
		return
	}

	message := buf[:numBytes]
	n.Logger.Printf("Received message from %s: %s", peerID, string(message))
}

// Shutdown gracefully shuts down the P2P network.
func (n *P2PNetwork) Shutdown() {
	n.Cancel()

	if err := n.Host.Close(); err != nil {
		n.Logger.Printf("Error closing host: %v", err)
	} else {
		n.Logger.Println("Host closed successfully.")
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
		n.Logger.Printf("Peer added: %s", id)
	}
}

// RemovePeer removes a peer from the internal peer list.
func (n *P2PNetwork) RemovePeer(id peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.Peers[id]; exists {
		delete(n.Peers, id)
		n.Logger.Printf("Peer removed: %s", id)
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
