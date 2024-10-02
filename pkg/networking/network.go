package networking

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/peerdns/peerdns/pkg/privacy"
	"github.com/pkg/errors"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
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

// Network defines the core structure of the P2P network.
type Network struct {
	Host           host.Host
	PubSub         *pubsub.PubSub
	Topic          *pubsub.Topic
	ProtocolID     protocol.ID
	Ctx            context.Context
	Cancel         context.CancelFunc
	PeersManager   *PeersManager
	cfg            config.Networking // Store the configuration in the P2PNetwork struct
	mu             sync.RWMutex
	Logger         logger.Logger
	PrivacyManager *privacy.PrivacyManager
	mdns           mdns.Service
	Metrics        *P2PMetrics
	Observability  *observability.Observability
}

// NewNetwork initializes and returns a new P2P network without starting it.
func NewNetwork(ctx context.Context, cfg config.Networking, did *identity.DID, logger logger.Logger, obs *observability.Observability) (*Network, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Create the libp2p host using the CreateHost function.
	libp2pHost, err := CreateHost(cfg, logger, did)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Initialize PeersManager and Metrics
	peersManager := NewPeersManager(logger)
	metrics, err := InitializeP2PMetrics(ctx, obs.Meter)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize P2P metrics: %w", err)
	}

	network := &Network{
		Host:          libp2pHost,
		PeersManager:  peersManager,
		ProtocolID:    protocol.ID(cfg.ProtocolID), // Use ProtocolID from config
		Ctx:           ctx,
		Cancel:        cancel,
		cfg:           cfg,
		Logger:        logger,
		Metrics:       metrics,
		Observability: obs,
	}

	return network, nil
}

// Start begins the P2P network's operations.
func (n *Network) Start() error {
	ps, err := pubsub.NewGossipSub(n.Ctx, n.Host)
	if err != nil {
		n.Logger.Error("Failed to create PubSub", zap.Error(err))
		return fmt.Errorf("failed to create pubsub: %w", err)
	}

	// Join or create a topic using ProtocolID from the configuration.
	topic, err := ps.Join(n.cfg.ProtocolID)
	if err != nil {
		n.Logger.Error("Failed to join topic", zap.Error(err))
		return fmt.Errorf("failed to join topic: %w", err)
	}

	n.PubSub = ps
	n.Topic = topic

	// Set up mDNS discovery if enabled.
	if n.cfg.EnableMDNS {
		mdnsService := mdns.NewMdnsService(n.Host, "peerdns-mdns", &MdnsNotifier{n: n})
		if err := mdnsService.Start(); err != nil {
			n.Logger.Error("Failed to start mDNS service", zap.Error(err))
		} else {
			n.mdns = mdnsService
			n.Logger.Info("mDNS service started")
		}
	}

	// Set the stream handler for the custom protocol.
	n.Host.SetStreamHandler(n.ProtocolID, n.handleStream)

	// Log the host information.
	n.Logger.Info("P2P network started",
		zap.String("protocolID", string(n.ProtocolID)),
		zap.Strings("listenAddresses", addrStrings(n.Host.Addrs())),
		zap.String("hostID", n.Host.ID().String()),
	)

	// Optionally connect to bootstrap peers if provided.
	for _, peerAddr := range n.cfg.BootstrapPeers {
		if err := n.ConnectPeer(peerAddr); err != nil {
			n.Logger.Warn("Failed to connect to bootstrap peer", zap.String("peer", peerAddr), zap.Error(err))
		} else {
			n.Logger.Info("Connected to bootstrap peer", zap.String("peer", peerAddr))
		}
	}

	return nil
}

// Shutdown gracefully shuts down the P2P network.
func (n *Network) Shutdown() error {
	n.Logger.Info("Shutting down P2P network")
	n.Cancel()

	// Close the mDNS service if it was started.
	if n.mdns != nil {
		if err := n.mdns.Close(); err != nil {
			return errors.Wrap(err, "failed to close mdns service")
		} else {
			n.Logger.Debug("mDNS service closed successfully")
		}
	}

	// Close the host and release resources.
	if err := n.Host.Close(); err != nil {
		return errors.Wrap(err, "failed to shut down network host")
	} else {
		n.Logger.Debug("Host closed successfully")
	}

	// Update metrics to reflect that the network is shutting down.
	activePeers := int64(len(n.PeersManager.GetPeers()))
	if activePeers > 0 {
		n.Metrics.RecordPeersRemoved(n.Ctx, activePeers)
		n.Metrics.RecordActivePeers(n.Ctx, -activePeers)
	}

	return nil
}

// handleStream processes incoming streams using the custom protocol.
func (n *Network) handleStream(s network.Stream) {
	defer s.Close()

	peerID := s.Conn().RemotePeer()
	n.Logger.Info("Received stream from peer", zap.String("peerID", peerID.String()))

	// Read the incoming message.
	buf := make([]byte, 4096)
	numBytes, err := s.Read(buf)
	if err != nil {
		n.Logger.Error("Error reading from stream", zap.Error(err), zap.String("peerID", peerID.String()))
		return
	}

	message := buf[:numBytes]
	startTime := time.Now()

	n.Logger.Info("Received message",
		zap.String("fromPeer", peerID.String()),
		zap.ByteString("message", message),
	)
	n.Metrics.RecordMessagesReceived(n.Ctx, 1)
	n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))
}

// ConnectPeerInfo connects to a peer using the provided peer.AddrInfo.
func (n *Network) ConnectPeerInfo(pi peer.AddrInfo) error {
	startTime := time.Now()
	if err := n.Host.Connect(n.Ctx, pi); err != nil {
		n.Logger.Warn("Failed to connect to peer", zap.String("peerID", pi.ID.String()), zap.Error(err))
		n.Metrics.RecordPeersConnectionFailed(n.Ctx, 1)
		n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))
		return fmt.Errorf("failed to connect to peer %s: %w", pi.ID, err)
	}

	n.PeersManager.AddPeer(pi.ID, pi.Addrs)
	n.Logger.Info("Connected to peer",
		zap.String("peerID", pi.ID.String()),
		zap.Strings("addresses", multiaddrsToStrings(pi.Addrs)),
	)
	n.Metrics.RecordPeersConnected(n.Ctx, 1)
	n.Metrics.RecordActivePeers(n.Ctx, 1)
	n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))

	return nil
}

// ConnectPeer connects to a given peer using its multiaddress.
func (n *Network) ConnectPeer(peerAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.Ctx, 1)
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.Ctx, 1)
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	n.Host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

	startTime := time.Now()
	if err := n.Host.Connect(n.Ctx, *peerInfo); err != nil {
		n.Logger.Warn("Failed to connect to peer", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
		n.Metrics.RecordPeersConnectionFailed(n.Ctx, 1)
		n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))
		return fmt.Errorf("failed to connect to peer %s: %w", peerInfo.ID, err)
	}

	n.PeersManager.AddPeer(peerInfo.ID, peerInfo.Addrs)
	n.Logger.Info("Connected to peer",
		zap.String("peerID", peerInfo.ID.String()),
		zap.Strings("addresses", multiaddrsToStrings(peerInfo.Addrs)),
	)
	n.Metrics.RecordPeersConnected(n.Ctx, 1)
	n.Metrics.RecordActivePeers(n.Ctx, 1)
	n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))

	return nil
}

// SendMessage sends a direct message to a specific peer using the custom protocol.
func (n *Network) SendMessage(target peer.ID, message []byte) error {
	startTime := time.Now()
	stream, err := n.Host.NewStream(n.Ctx, target, n.ProtocolID)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.Ctx, 1)
		n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	_, err = stream.Write(message)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.Ctx, 1)
		n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))
		return fmt.Errorf("failed to write message: %w", err)
	}

	n.Logger.Info("Sent message", zap.String("toPeer", target.String()), zap.ByteString("message", message))
	n.Metrics.RecordMessagesSent(n.Ctx, 1)
	n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))

	return nil
}

// GetPeers returns a list of currently connected peers.
func (n *Network) GetPeers() []*PeerInfo {
	return n.PeersManager.GetPeers()
}

// BroadcastMessage sends a message to all peers subscribed to the PubSub topic.
func (n *Network) BroadcastMessage(message []byte) error {
	startTime := time.Now()
	err := n.Topic.Publish(n.Ctx, message)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.Ctx, 1)
		n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	n.Logger.Info("Broadcasted message", zap.ByteString("message", message))
	n.Metrics.RecordMessagesSent(n.Ctx, 1)
	n.Metrics.RecordMessageLatency(n.Ctx, time.Since(startTime))

	return nil
}
