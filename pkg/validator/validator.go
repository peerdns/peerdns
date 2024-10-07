// pkg/validator/validator.go
package validator

import (
	"context"
	"fmt"
	"github.com/peerdns/peerdns/pkg/chain"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/metrics"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/peerdns/peerdns/pkg/storage"
	"time"

	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Validator represents an individual validator participating in the consensus.
type Validator struct {
	ctx          context.Context
	cancel       context.CancelFunc
	logger       logger.Logger
	cfg          *config.Config
	did          *accounts.DID
	vSet         *consensus.ValidatorSet
	shardManager *sharding.ShardManager
	sm           *storage.Manager
	obs          *observability.Observability
	network      *networking.Network
	consensus    *Consensus
	stateMgr     *StateManager
	collector    *metrics.Collector
	blockchain   *chain.Blockchain
}

// NewValidator initializes a new Validator.
// Assumes that the ValidatorSet is already populated with all validators.
func NewValidator(ctx context.Context, cfg *config.Config, did *accounts.DID, sm *storage.Manager, network *networking.Network, validatorSet *consensus.ValidatorSet, logger logger.Logger, obs *observability.Observability, collector *metrics.Collector, blockchain *chain.Blockchain) (*Validator, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Initialize state manager
	stateMgr := NewStateManager(logger, obs)

	// Set initial state as Uninitialized
	stateMgr.SetState(ValidatorStateType, Uninitialized)

	// Check if the ValidatorSet contains the peer ID for this validator
	if !validatorSet.IsValidator(did.PeerID) {
		logger.Error("Validator not found in ValidatorSet", zap.String("peerID", did.PeerID.String()))
		cancel()
		return nil, fmt.Errorf("validator not found for peer ID %s", did.PeerID.String())
	}

	// Initialize sharding
	shardManager := sharding.NewShardManager(cfg.Sharding.ShardCount, logger)

	// Assign validator to a shard
	shardID, shardErr := shardManager.AssignValidator(did.PeerID)
	if shardErr != nil {
		cancel()
		return nil, errors.Wrap(shardErr, "failed to assign validator to shard")
	}

	logger.Info("Validator assigned to shard",
		zap.String("validatorID", did.PeerID.String()),
		zap.Int("shardID", shardID),
	)

	consensusDb, err := sm.GetDb("consensus")
	if err != nil {
		logger.Error("Failed to get consensus database", zap.Error(err))
		cancel()
		return nil, err
	}

	vConsensus := NewConsensus(
		ctx, did, network, shardManager, consensusDb.(*storage.Db),
		logger, validatorSet, collector, stateMgr,
	)

	stateMgr.SetState(ValidatorStateType, Initialized)

	return &Validator{
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
		cfg:          cfg,
		did:          did,
		vSet:         validatorSet,
		shardManager: shardManager,
		consensus:    vConsensus,
		stateMgr:     stateMgr,
		blockchain:   blockchain,
	}, nil
}

func (v *Validator) StateManager() *StateManager {
	return v.stateMgr
}

func (v *Validator) Start() error {
	v.logger.Info("Starting validator services")
	v.stateMgr.SetState(ValidatorStateType, Starting)

	// Start the consensus itself...
	// @TODO: Probably not a good idea if node itself is not staked and have no access.
	// We should ensure that this is really covered to minimise safety risk.
	v.consensus.Start()

	// Update state to `Started` after successful startup
	v.stateMgr.SetState(ValidatorStateType, Started)

	return nil
}

func (v *Validator) Shutdown() error {
	v.stateMgr.SetState(ValidatorStateType, Stopping)
	v.cancel()

	// Shutdown components
	if err := v.consensus.Shutdown(); err != nil {
		v.stateMgr.SetState(ConsensusStateType, Failed)
		v.logger.Error("Error shutting down Consensus Module", zap.Error(err))
	}

	// Await for consensus to be fully started...
	if err := v.stateMgr.WaitForState(ConsensusStateType, Stopped, 10*time.Second); err != nil {
		v.stateMgr.SetState(ValidatorStateType, Failed)
		return errors.Wrap(err, "failed to wait for consensus started state")
	}

	v.logger.Info("Validator shutdown complete")

	// Set the state to Stopped after shutting down.
	v.stateMgr.SetState(ValidatorStateType, Stopped)
	return nil
}
