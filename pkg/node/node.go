// pkg/node/node.go
package node

import (
	"context"
	"sync"

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
	dids, err := identityManager.ListAllDIDs()
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

	// Initialize the validator
	validatorInstance, vErr := validator.NewValidator(validatorDID, shardManager, logger)
	if vErr != nil {
		logger.Error("Failed to initialize validator", zap.Error(vErr))
		cancel()
		return nil, vErr
	}

	// Initialize the blockchain
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

func (n *Node) Start() {
	n.Logger.Info("Starting node")
	n.SetState(NodeStateStarting)

	// Start the consensus module
	n.Consensus.Start()

	n.SetState(NodeStateRunning)
}

func (n *Node) Shutdown() {
	n.SetState(NodeStateStopping)

	n.Cancel()

	// Shutdown components
	n.Consensus.Shutdown()
	n.Network.Shutdown()
	n.StorageManager.Close()
	n.Logger.Info("Node shutdown complete")

	n.SetState(NodeStateStopped)
}
