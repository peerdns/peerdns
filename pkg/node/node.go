package node

import (
	"context"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/genesis"
	"github.com/peerdns/peerdns/pkg/ledger"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/pkg/errors"
	"time"

	"github.com/peerdns/peerdns/pkg/chain"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/metrics"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/validator"
	"go.uber.org/zap"
)

// Node encapsulates all components of a PeerDNS node.
type Node struct {
	logger         logger.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	im             *accounts.Manager
	validator      *validator.Validator
	ledger         *ledger.Ledger
	chain          *chain.Blockchain
	network        *networking.Network
	shardManager   *sharding.ShardManager
	storageManager *storage.Manager
	discovery      *networking.DiscoveryService
	collector      *metrics.Collector
	pm             *metrics.PerformanceMonitor
	stateMgr       *StateManager
}

// NewNode initializes and returns a new Node.
func NewNode(ctx context.Context, config *config.Config, logger logger.Logger, sm *storage.Manager, im *accounts.Manager, obs *observability.Observability) (*Node, error) {
	// Create a child context for the node
	nodeCtx, cancel := context.WithCancel(ctx)

	// Initialize state manager
	stateMgr := NewStateManager(logger, obs)

	// Set initial state as Uninitialized
	stateMgr.SetState(NodeStateType, Uninitialized)

	// Ensure the PeerID is set in the configuration
	if config.Networking.PeerID == "" {
		logger.Error("PeerID not provided in the networking configuration")
		cancel()
		return nil, errors.New("peerId must be defined in the networking configuration")
	}

	// Attempt to load the identity from the manager using the PeerID
	did, err := im.GetByPeerID(config.Networking.PeerID)
	if err != nil {
		logger.Error("Failed to load DID from identity manager", zap.String("PeerID", config.Networking.PeerID.String()), zap.Error(err))
		cancel()
		return nil, errors.Wrapf(err, "failed to load DID from identity manager with PeerID: %s", config.Networking.PeerID)
	}

	logger.Info("Loaded node identity", zap.String("PeerID", did.PeerID.String()))

	// Get ledger here, we may need it within networking, shard manager and what not so good to be here...
	chainDb, cdbErr := sm.GetDb("chain")
	if cdbErr != nil {
		cancel()
		return nil, errors.Wrap(cdbErr, "failed to get chain database")
	}

	ldgr, ldgrErr := ledger.NewLedger(nodeCtx, chainDb)
	if ldgrErr != nil {
		cancel()
		logger.Error("Failed to initialize new ledger", zap.Error(ldgrErr))
		return nil, errors.Wrap(ldgrErr, "failed to initialize new ledger")
	}

	// Initialize networking
	ntwrk, err := networking.NewNetwork(nodeCtx, config.Networking, did, logger, obs)
	if err != nil {
		logger.Error("Failed to initialize P2P network", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize sharding
	shardManager := sharding.NewShardManager(config.Sharding.ShardCount, logger)

	// Initialize the ValidatorSet
	vSet := consensus.NewValidatorSet(logger)

	// Add this node to the validator set
	// @TODO: This cannot be just as simple as this... Has to go through consensus...
	// Consensus itself has to have mechanism that if anyone does this, will be rejected regardless.
	vSet.AddValidator(did)

	// Initialize the DiscoveryService
	bootstrapAddrs, err := config.Networking.BootstrapPeersAsAddrs()
	if err != nil {
		logger.Error("Failed to parse bootstrap peers", zap.Error(err))
		cancel()
		return nil, err
	}

	discoveryService, err := networking.NewDiscoveryService(nodeCtx, ntwrk.Host, logger, bootstrapAddrs, obs)
	if err != nil {
		logger.Error("Failed to initialize discovery service", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize MetricsCollector with default weights
	// @TODO: These weights severely needs to be researched later on
	weights := metrics.Metrics{
		BandwidthUsage: 0.0,
		Computational:  0.0,
		Storage:        0.0,
		Uptime:         1.0,
		Responsiveness: 0.0,
		Reliability:    0.0,
	}
	collector := metrics.NewCollector(weights)

	// Initialize PerformanceMonitor (do not start it yet)
	performanceMonitor := metrics.NewPerformanceMonitor(nodeCtx, ntwrk.Host, ntwrk.ProtocolID, logger, obs, collector, 5*time.Second, 5, 10*time.Second)

	// Load the genesis configuration
	genesisConfig, err := genesis.LoadGenesis(config.Genesis.Path)
	if err != nil {
		logger.Error("Failed to load genesis configuration", zap.Error(err))
		cancel()
		return nil, errors.Wrap(err, "failed to load genesis configuration")
	}

	// Initialize blockchain
	blockchain, bErr := chain.NewBlockchain(nodeCtx, logger, ldgr, genesisConfig)
	if bErr != nil {
		cancel()
		return nil, errors.Wrap(bErr, "failure to create new blockchain instance")
	}

	// Create the validator instance
	nodeValidator, err := validator.NewValidator(ctx, config, did, sm, ntwrk, vSet, logger, obs, collector, blockchain)
	if err != nil {
		logger.Error("Failed to initialize validator", zap.Error(err))
		cancel()
		return nil, err
	}

	// Create the node instance
	node := &Node{
		im:             im,
		validator:      nodeValidator,
		ledger:         ldgr,
		chain:          blockchain,
		network:        ntwrk,
		shardManager:   shardManager,
		storageManager: sm,
		discovery:      discoveryService,
		collector:      collector,
		pm:             performanceMonitor,
		logger:         logger,
		ctx:            nodeCtx,
		cancel:         cancel,
		stateMgr:       stateMgr,
	}

	// Set the state to Initialized after successful node creation.
	node.stateMgr.SetState(NodeStateType, Initialized)

	return node, nil
}

func (n *Node) IdentityManager() *accounts.Manager {
	return n.im
}

func (n *Node) StateManager() *StateManager {
	return n.stateMgr
}

func (n *Node) ShardManager() *sharding.ShardManager {
	return n.shardManager
}

func (n *Node) StorageManager() *storage.Manager {
	return n.storageManager
}

func (n *Node) DiscoveryService() *networking.DiscoveryService {
	return n.discovery
}

func (n *Node) Collector() *metrics.Collector {
	return n.collector
}

func (n *Node) PerformanceMonitor() *metrics.PerformanceMonitor {
	return n.pm
}

func (n *Node) Validator() *validator.Validator {
	return n.validator
}

// Start begins the node's operations.
func (n *Node) Start() error {
	n.logger.Info("Starting node")
	n.stateMgr.SetState(NodeStateType, Starting)

	if err := n.network.Start(); err != nil {
		return errors.Wrap(err, "failure to start P2P network")
	}

	// Start a goroutine to continuously advertise the validator service
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := n.discovery.Advertise("validator")
				if err != nil {
					n.logger.Warn("Failed to advertise validator service", zap.Error(err))
				} else {
					n.logger.Info("Successfully advertised validator service")
					return
				}
			case <-n.ctx.Done():
				return
			}
		}
	}()

	// Register stream handler for ping/pong
	n.network.Host.SetStreamHandler(n.network.ProtocolID, func(s network.Stream) {
		n.logger.Debug(
			"Received new stream",
			zap.String("protocol", string(s.Protocol())),
			zap.String("from", s.Conn().RemotePeer().String()),
		)
		defer s.Close()

		buf := make([]byte, 1024)
		nBytes, err := s.Read(buf)
		if err != nil {
			n.logger.Warn("Error reading from stream", zap.Error(err))
			return
		}

		request := string(buf[:nBytes])
		n.logger.Debug(
			"Received request",
			zap.String("request", request),
			zap.String("from", s.Conn().RemotePeer().String()),
		)

		if request == "ping" {
			_, err := s.Write([]byte("pong"))
			if err != nil {
				n.logger.Warn("Error writing to stream", zap.Error(err))
			} else {
				n.logger.Info("Responded with pong", zap.String("to", s.Conn().RemotePeer().String()))
			}
		}
	})

	// Start the performance monitor
	n.pm.Start()

	// TODO: This portion here should be started with consensus service, not node itself here...
	{
		// Start the consensus module
		if err := n.validator.Start(); err != nil {
			return errors.Wrap(err, "failed to start validator")
		}

		// Await for consensus to be fully started...
		if err := n.validator.StateManager().WaitForState(validator.ValidatorStateType, validator.Started, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for validator started state")
		}
	}

	// Set the state to Started after successfully starting all components.
	n.stateMgr.SetState(NodeStateType, Started)
	return nil
}

// Shutdown gracefully shuts down the node.
func (n *Node) Shutdown() error {
	n.stateMgr.SetState(NodeStateType, Stopping)

	n.cancel()

	// Stop the PerformanceMonitor
	if n.pm != nil {
		n.pm.Stop()
	}

	// Shutdown components
	if err := n.validator.Shutdown(); err != nil {
		n.stateMgr.SetState(ConsensusStateType, Failed)
		n.logger.Error("Error shutting down Consensus Module", zap.Error(err))
	}

	// Await for consensus to be fully started...
	if err := n.validator.StateManager().WaitForState(validator.ValidatorStateType, validator.Stopped, 10*time.Second); err != nil {
		n.stateMgr.SetState(NodeStateType, Failed)
		return errors.Wrap(err, "failed to wait for consensus started state")
	}

	if err := n.network.Shutdown(); err != nil {
		n.stateMgr.SetState(NodeStateType, Failed)
		return err
	}

	if err := n.storageManager.Close(); err != nil {
		n.stateMgr.SetState(NodeStateType, Failed)
		return errors.Wrap(err, "failed to close storage manager")
	}

	n.logger.Info("Node shutdown complete")

	// Set the state to Stopped after shutting down.
	n.stateMgr.SetState(NodeStateType, Stopped)
	return nil
}
