// pkg/node/node.go
package node

import (
	"context"
	"log"

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
	Logger          *logger.Logger
	Ctx             context.Context
	Cancel          context.CancelFunc
}

func NewNode(ctx context.Context, config config.Config) (*Node, error) {
	// Create a child context for the node
	ctx, cancel := context.WithCancel(ctx)

	// Initialize the logger
	if err := logger.InitializeGlobalLogger(config.Logger); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	appLogger := logger.G()

	// Initialize storage manager
	storageManager, err := storage.NewManager(ctx, config.Mdbx)
	if err != nil {
		appLogger.Error("Failed to create storage manager", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize identity manager
	identityDb, err := storageManager.GetDb("identity")
	if err != nil {
		appLogger.Error("Failed to get identity database", zap.Error(err))
		cancel()
		return nil, err
	}
	identityManager := identity.NewManager(identityDb.(*storage.Db))

	// Load or create validator identity
	dids, err := identityManager.ListAllDIDs()
	if err != nil {
		appLogger.Error("Failed to list DIDs", zap.Error(err))
		cancel()
		return nil, err
	}

	var validatorDID *identity.DID
	if len(dids) > 0 {
		validatorDID = dids[0] // Use the first DID
		appLogger.Info("Loaded existing validator DID", zap.String("DID", validatorDID.ID))
	} else {
		validatorDID, err = identityManager.CreateNewDID()
		if err != nil {
			appLogger.Error("Failed to create new DID", zap.Error(err))
			cancel()
			return nil, err
		}
		appLogger.Info("Created new validator DID", zap.String("DID", validatorDID.ID))
	}

	// Initialize networking
	network, err := networking.NewP2PNetwork(ctx, config.Networking.ListenPort, config.Networking.ProtocolID, appLogger)
	if err != nil {
		appLogger.Error("Failed to initialize P2P network", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize privacy manager
	privacyManager, err := privacy.NewPrivacyManager()
	if err != nil {
		appLogger.Error("Failed to initialize privacy manager", zap.Error(err))
		cancel()
		return nil, err
	}
	network.PrivacyManager = privacyManager

	// Initialize sharding
	shardManager := sharding.NewShardManager(config.Sharding.ShardCount, appLogger)

	// Initialize the validator
	validatorInstance := validator.NewValidator(validatorDID, shardManager, appLogger)

	// Initialize the blockchain
	chainDb, err := storageManager.GetDb("chain")
	if err != nil {
		appLogger.Error("Failed to get chain database", zap.Error(err))
		cancel()
		return nil, err
	}
	blockchain := chain.NewBlockchain(chainDb, appLogger)

	// Initialize consensus module
	consensusDb, err := storageManager.GetDb("consensus")
	if err != nil {
		appLogger.Error("Failed to get consensus database", zap.Error(err))
		cancel()
		return nil, err
	}
	consensusModule := NewConsensusModule(ctx, validatorDID, network, shardManager, privacyManager, consensusDb, appLogger, validatorInstance)

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
		Logger:          appLogger,
		Ctx:             ctx,
		Cancel:          cancel,
	}

	return node, nil
}

func (n *Node) Start() {
	n.Logger.Info("Starting node")

	// Start the network (if needed)
	// n.Network.Start() // If your existing networking code requires starting

	// Start the consensus module
	n.Consensus.Start()
}

func (n *Node) Shutdown() {
	n.Cancel()

	// Shutdown components
	n.Consensus.Shutdown()
	n.Network.Shutdown()
	n.StorageManager.Close()
	n.Logger.Info("Node shutdown complete")
}
