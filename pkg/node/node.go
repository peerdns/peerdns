// pkg/node/node.go
package node

import (
	"context"
	"github.com/libp2p/go-libp2p/core/network"
	"sync"
	"time"

	"github.com/peerdns/peerdns/pkg/chain"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/metrics"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/privacy"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/validator"
	"go.uber.org/zap"
)

// Node encapsulates all components of a Peerdns node.
type Node struct {
	IdentityManager    *identity.Manager
	Validator          *validator.Validator
	Chain              *chain.Blockchain
	Network            *networking.P2PNetwork
	Consensus          *Consensus
	ShardManager       *sharding.ShardManager
	PrivacyManager     *privacy.PrivacyManager
	StorageManager     *storage.Manager
	Discovery          *networking.DiscoveryService
	MetricsCollector   *metrics.MetricsCollector
	PerformanceMonitor *metrics.PerformanceMonitor
	Logger             logger.Logger
	Ctx                context.Context
	Cancel             context.CancelFunc

	state     NodeState
	stateLock sync.RWMutex
}

// NewNode initializes and returns a new Node.
func NewNode(ctx context.Context, config config.Config, logger logger.Logger) (*Node, error) {
	// Create a child context for the node
	nodeCtx, cancel := context.WithCancel(ctx)

	logger.Info("Starting up the node")

	// Initialize storage manager
	storageManager, err := storage.NewManager(nodeCtx, config.Mdbx)
	if err != nil {
		logger.Error("Failed to create storage manager", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize identity manager
	identityManager, imErr := identity.NewManager(&config.Identity, logger)
	if imErr != nil {
		logger.Error("Failed to create identity manager", zap.Error(imErr))
		cancel()
		return nil, imErr
	}

	// Load existing identities or create a new one if none exist
	err = identityManager.Load()
	if err != nil {
		logger.Error("Failed to load identities", zap.Error(err))
		cancel()
		return nil, err
	}

	var selfDID *identity.DID
	dids, err := identityManager.List()
	if err != nil {
		logger.Error("Failed to list DIDs", zap.Error(err))
		cancel()
		return nil, err
	}

	if len(dids) == 0 {
		// No existing DIDs found, create a new one
		selfDID, err = identityManager.Create("NodeIdentity", "Primary node identity", true)
		if err != nil {
			logger.Error("Failed to create new DID", zap.Error(err))
			cancel()
			return nil, err
		}
		logger.Info("Created new node DID", zap.String("DID", selfDID.ID))
	} else {
		// Use the first existing DID
		selfDID = dids[0]
		logger.Info("Loaded existing node DID", zap.String("DID", selfDID.ID))
	}

	// Initialize networking
	network, err := networking.NewP2PNetwork(nodeCtx, config.Networking, logger)
	if err != nil {
		logger.Error("Failed to initialize P2P network", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize privacy manager
	privacyManager, err := privacy.NewPrivacyManager()
	if err != nil {
		logger.Error("Failed to initialize privacy manager", zap.Error(err))
		cancel()
		return nil, err
	}
	network.PrivacyManager = privacyManager

	// Initialize sharding
	shardManager := sharding.NewShardManager(config.Sharding.ShardCount, logger)

	// Initialize the ValidatorSet
	validatorSet := consensus.NewValidatorSet(logger)

	// Add your own node to the ValidatorSet
	validatorSet.AddValidator(selfDID.PeerID, selfDID.SigningPublicKey, selfDID.SigningPrivateKey)

	// Initialize the DiscoveryService
	bootstrapAddrs, err := config.Networking.BootstrapPeersAsAddrs()
	if err != nil {
		logger.Error("Failed to parse bootstrap peers", zap.Error(err))
		cancel()
		return nil, err
	}

	discoveryService, err := networking.NewDiscoveryService(nodeCtx, network.Host, logger, bootstrapAddrs)
	if err != nil {
		logger.Error("Failed to initialize discovery service", zap.Error(err))
		cancel()
		return nil, err
	}

	// Start a goroutine to continuously advertise the validator service
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := discoveryService.Advertise("validator")
				if err != nil {
					logger.Warn("Failed to advertise validator service", zap.Error(err))
				} else {
					logger.Info("Successfully advertised validator service")
					return // Exit the goroutine once successful
				}
			case <-nodeCtx.Done():
				return
			}
		}
	}()

	// Initialize MetricsCollector with weights
	weights := metrics.Metrics{
		BandwidthUsage: 1.0,
		Computational:  0.8,
		Storage:        0.6,
		Uptime:         1.0,
		Responsiveness: 0.9,
		Reliability:    1.0,
	}
	metricsCollector := metrics.NewMetricsCollector(weights)

	// Initialize PerformanceMonitor (do not start it yet)
	performanceMonitor := metrics.NewPerformanceMonitor(nodeCtx, network.Host, network.ProtocolID, logger, metricsCollector, 5*time.Second, 5, 10*time.Second)

	// Initialize blockchain
	chainDb, err := storageManager.GetDb("chain")
	if err != nil {
		logger.Error("Failed to get chain database", zap.Error(err))
		cancel()
		return nil, err
	}
	blockchain := chain.NewBlockchain(chainDb.(*storage.Db), logger)

	// Initialize consensus module
	consensusDb, err := storageManager.GetDb("consensus")
	if err != nil {
		logger.Error("Failed to get consensus database", zap.Error(err))
		cancel()
		return nil, err
	}

	// Create the validator instance
	nodeValidator, err := validator.NewValidator(selfDID, shardManager, validatorSet, logger)
	if err != nil {
		logger.Error("Failed to initialize validator", zap.Error(err))
		cancel()
		return nil, err
	}

	// Create the consensus module
	consensusModule := NewConsensus(nodeCtx, selfDID, network, shardManager, privacyManager, consensusDb.(*storage.Db), logger, nodeValidator, metricsCollector)

	// Create the node instance
	node := &Node{
		IdentityManager:    identityManager,
		Validator:          nodeValidator,
		Chain:              blockchain,
		Network:            network,
		Consensus:          consensusModule,
		ShardManager:       shardManager,
		PrivacyManager:     privacyManager,
		StorageManager:     storageManager,
		Discovery:          discoveryService,
		MetricsCollector:   metricsCollector,
		PerformanceMonitor: performanceMonitor,
		Logger:             logger,
		Ctx:                nodeCtx,
		Cancel:             cancel,
		state:              NodeStateStopped,
	}

	return node, nil
}

// Start begins the node's operations.
func (n *Node) Start() {
	n.Logger.Info("Starting node")
	n.SetState(NodeStateStarting)

	// Register stream handler for ping/pong
	n.Network.Host.SetStreamHandler(n.Network.ProtocolID, func(s network.Stream) {
		n.Logger.Info("Received new stream", zap.String("protocol", string(s.Protocol())), zap.String("from", s.Conn().RemotePeer().String()))
		defer s.Close()
		buf := make([]byte, 1024)
		nBytes, err := s.Read(buf)
		if err != nil {
			n.Logger.Warn("Error reading from stream", zap.Error(err))
			return
		}
		request := string(buf[:nBytes])
		n.Logger.Info("Received request", zap.String("request", request), zap.String("from", s.Conn().RemotePeer().String()))
		if request == "ping" {
			// Respond with "pong"
			_, err := s.Write([]byte("pong"))
			if err != nil {
				n.Logger.Warn("Error writing to stream", zap.Error(err))
			} else {
				n.Logger.Info("Responded with pong", zap.String("to", s.Conn().RemotePeer().String()))
			}
		} else {
			n.Logger.Warn("Received unknown request", zap.String("request", request))
		}
	})

	// Start the performance monitor
	n.PerformanceMonitor.Start()

	// Start the consensus module
	n.Consensus.Start()

	n.SetState(NodeStateRunning)
}

// Shutdown gracefully shuts down the node.
func (n *Node) Shutdown() error {
	n.SetState(NodeStateStopping)

	n.Cancel()

	// Stop the PerformanceMonitor
	if n.PerformanceMonitor != nil {
		n.PerformanceMonitor.Stop()
	}

	// Shutdown components
	if err := n.Consensus.Shutdown(); err != nil {
		n.Logger.Error("Error shutting down Consensus Module", zap.Error(err))
	}
	n.Network.Shutdown()
	n.StorageManager.Close()
	n.Logger.Info("Node shutdown complete")

	n.SetState(NodeStateStopped)
	return nil
}

// SetState sets the current state of the node in a thread-safe manner.
func (n *Node) SetState(state NodeState) {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()
	n.state = state
}

// GetState retrieves the current state of the node in a thread-safe manner.
func (n *Node) GetState() NodeState {
	n.stateLock.RLock()
	defer n.stateLock.RUnlock()
	return n.state
}
