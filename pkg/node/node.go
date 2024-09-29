// pkg/node/node.go
package node

import (
	"context"
	"github.com/peerdns/peerdns/pkg/consensus"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer" // Added import
	"github.com/peerdns/peerdns/pkg/chain"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/privacy"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/validator"
	"go.uber.org/zap"
)

type Node struct {
	IdentityManager *identity.Manager
	Validator       *validator.Validator
	Chain           *chain.Blockchain
	Network         *networking.P2PNetwork
	Consensus       *ConsensusModule
	ShardManager    *sharding.ShardManager
	PrivacyManager  *privacy.PrivacyManager
	StorageManager  *storage.Manager
	Logger          logger.Logger
	Ctx             context.Context
	Cancel          context.CancelFunc

	state     NodeState
	stateLock sync.RWMutex
}

func NewNode(ctx context.Context, config config.Config, logger logger.Logger) (*Node, error) {
	// Create a child context for the node
	ctx, cancel := context.WithCancel(ctx)

	logger.Info("Starting up the node")

	// Initialize storage manager
	storageManager, err := storage.NewManager(ctx, config.Mdbx)
	if err != nil {
		logger.Error("Failed to create storage manager", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize identity manager
	identityDb, err := storageManager.GetDb("identity")
	if err != nil {
		logger.Error("Failed to get identity database", zap.Error(err))
		cancel()
		return nil, err
	}
	identityManager := identity.NewManager(identityDb.(*storage.Db))

	// Load or create validator identity
	dids, err := identityManager.ListAllDIDs(ctx)
	if err != nil {
		logger.Error("Failed to list DIDs", zap.Error(err))
		cancel()
		return nil, err
	}

	var validatorDID *identity.DID
	if len(dids) > 0 {
		validatorDID = dids[0] // Use the first DID
		logger.Info("Loaded existing validator DID", zap.String("DID", validatorDID.ID))
	} else {
		validatorDID, err = identityManager.CreateNewDID()
		if err != nil {
			logger.Error("Failed to create new DID", zap.Error(err))
			cancel()
			return nil, err
		}
		logger.Info("Created new validator DID", zap.String("DID", validatorDID.ID))
	}

	// Initialize networking
	network, err := networking.NewP2PNetwork(ctx, config.Networking.ListenPort, config.Networking.ProtocolID, logger)
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
	// TODO: Add all validators to the ValidatorSet
	// For now, assuming self is the only validator
	// If you have other validators, add them here.

	validatorInstance := &validator.Validator{
		ValidatorSet:  validatorSet,
		ValidatorInfo: &consensus.Validator{ID: peer.ID("")}, // Initialize properly
	}

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
	consensusModule := NewConsensusModule(ctx, validatorDID, network, shardManager, privacyManager, consensusDb.(*storage.Db), logger, validatorInstance)

	// Create the node instance
	node := &Node{
		IdentityManager: identityManager,
		Validator:       validatorInstance,
		Chain:           blockchain,
		Network:         network,
		Consensus:       consensusModule,
		ShardManager:    shardManager,
		PrivacyManager:  privacyManager,
		StorageManager:  storageManager,
		Logger:          logger,
		Ctx:             ctx,
		Cancel:          cancel,
		state:           NodeStateStopped,
	}

	return node, nil
}

func (n *Node) GetStorageManager() *storage.Manager {
	return n.StorageManager
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

// Start begins the node's operations.
func (n *Node) Start() {
	n.Logger.Info("Starting node")
	n.SetState(NodeStateStarting)

	// Start the consensus module
	n.Consensus.Start()

	n.SetState(NodeStateRunning)
}

// Shutdown gracefully shuts down the node.
func (n *Node) Shutdown() error {
	n.SetState(NodeStateStopping)

	n.Cancel()

	// Shutdown components
	if err := n.Consensus.Shutdown(); err != nil {
		n.Logger.Error("Error shutting down Consensus Module", zap.Error(err))
	}
	n.Network.Shutdown()
	n.StorageManager.Close()
	n.Logger.Info("Node shutdown complete")

	n.SetState(NodeStateStopped)
	return nil // Return error if any occurred during shutdown
}
