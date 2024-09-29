// pkg/validator/validator.go
package validator

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/sharding"
	"go.uber.org/zap"
)

type Validator struct {
	DID           *identity.DID
	ValidatorSet  *consensus.ValidatorSet
	ShardManager  *sharding.ShardManager
	ValidatorInfo *consensus.Validator
	Logger        logger.Logger
}

func NewValidator(did *identity.DID, shardManager *sharding.ShardManager, logger logger.Logger) (*Validator, error) {
	validatorSet := consensus.NewValidatorSet(logger)
	peerID := peer.ID(did.ID)

	validatorSet.AddValidator(peerID, did.PublicKey, did.PrivateKey)

	// Assign validator to a shard
	shardID, shardErr := shardManager.AssignValidator(peerID)
	if shardErr != nil {
		return nil, shardErr
	}

	logger.Info("Validator assigned to shard", zap.String("validatorID", peerID.String()), zap.Int("shardID", shardID))

	return &Validator{
		DID:           did,
		ValidatorSet:  validatorSet,
		ShardManager:  shardManager,
		ValidatorInfo: validatorSet.GetValidator(peerID),
		Logger:        logger,
	}, nil
}
