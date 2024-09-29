// pkg/validator/validator.go
package validator

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/sharding"
	"go.uber.org/zap"
)

type Validator struct {
	DID           *identity.DID
	ValidatorSet  *consensus.ValidatorSet
	ShardManager  *sharding.ShardManager
	ValidatorInfo *consensus.Validator
	Logger        *zap.Logger
}

func NewValidator(did *identity.DID, shardManager *sharding.ShardManager, logger *zap.Logger) *Validator {
	validatorSet := consensus.NewValidatorSet(logger, nil)
	peerID := peer.ID(did.ID)

	validatorSet.AddValidator(peerID, did.PublicKey, did.PrivateKey)

	// Assign validator to a shard
	shardID := shardManager.AssignValidator(peerID)
	logger.Info("Validator assigned to shard", zap.String("validatorID", peerID.String()), zap.Int("shardID", shardID))

	return &Validator{
		DID:           did,
		ValidatorSet:  validatorSet,
		ShardManager:  shardManager,
		ValidatorInfo: validatorSet.GetValidator(peerID),
		Logger:        logger,
	}
}
