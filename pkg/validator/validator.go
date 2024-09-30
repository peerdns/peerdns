// pkg/validator/validator.go
package validator

import (
	"fmt"

	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Validator represents an individual validator participating in the consensus.
type Validator struct {
	DID          *identity.DID
	ValidatorSet *consensus.ValidatorSet
	ShardManager *sharding.ShardManager
	Logger       logger.Logger
}

// NewValidator initializes a new Validator.
// Assumes that the ValidatorSet is already populated with all validators.
func NewValidator(did *identity.DID, shardManager *sharding.ShardManager, validatorSet *consensus.ValidatorSet, logger logger.Logger) (*Validator, error) {
	peerID := did.PeerID

	// Check if the ValidatorSet contains the peer ID for this validator
	if !validatorSet.IsValidator(peerID) {
		logger.Error("Validator not found in ValidatorSet", zap.String("peerID", peerID.String()))
		return nil, fmt.Errorf("validator not found for peer ID %s", peerID.String())
	}

	// Assign validator to a shard
	shardID, shardErr := shardManager.AssignValidator(peerID)
	if shardErr != nil {
		return nil, errors.Wrap(shardErr, "failed to assign validator to shard")
	}

	logger.Info("Validator assigned to shard",
		zap.String("validatorID", peerID.String()),
		zap.Int("shardID", shardID),
	)

	return &Validator{
		DID:          did,
		ValidatorSet: validatorSet,
		ShardManager: shardManager,
		Logger:       logger,
	}, nil
}
