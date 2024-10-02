package sharding

import (
	"errors"
	"fmt"
	"github.com/peerdns/peerdns/pkg/logger"
	"hash/fnv"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

// ShardManager manages all shards in the network.
type ShardManager struct {
	ShardCount int
	Shards     map[int]*Shard

	mu     sync.RWMutex
	logger logger.Logger
}

// NewShardManager initializes a new ShardManager with the specified number of shards.
func NewShardManager(shardCount int, logger logger.Logger) *ShardManager {
	shards := make(map[int]*Shard)
	for i := 0; i < shardCount; i++ {
		shards[i] = NewShard(i)
	}

	return &ShardManager{
		ShardCount: shardCount,
		Shards:     shards,
		logger:     logger,
	}
}

// AssignValidator assigns a validator to a shard using consistent hashing.
func (sm *ShardManager) AssignValidator(validatorID peer.ID) (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if validator is already assigned
	for shardID, shard := range sm.Shards {
		if shard.HasValidator(validatorID) {
			return shardID, nil
		}
	}

	// Assign validator to a shard based on hash
	shardID := sm.hashValidator(validatorID) % sm.ShardCount
	shard := sm.Shards[shardID]
	shard.AddValidator(validatorID)

	sm.logger.Info("Validator assigned to shard",
		zap.String("validatorID", validatorID.String()),
		zap.Int("shardID", shardID),
	)

	return shardID, nil
}

// RemoveValidator removes a validator from its assigned shard.
func (sm *ShardManager) RemoveValidator(validatorID peer.ID) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find the shard containing the validator
	for _, shard := range sm.Shards {
		if shard.HasValidator(validatorID) {
			shard.RemoveValidator(validatorID)
			sm.logger.Info("Validator removed from shard",
				zap.String("validatorID", validatorID.String()),
				zap.Int("shardID", shard.ID),
			)
			return nil
		}
	}

	return fmt.Errorf("validator %s is not assigned to any shard", validatorID)
}

// GetValidatorsInShard returns the validators assigned to a specific shard.
func (sm *ShardManager) GetValidatorsInShard(shardID int) ([]peer.ID, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if shard, exists := sm.Shards[shardID]; exists {
		return shard.ListValidators(), nil
	}

	return nil, errors.New("invalid shard ID")
}

// GetShard returns the shard by its ID.
func (sm *ShardManager) GetShard(shardID int) (*Shard, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if shard, exists := sm.Shards[shardID]; exists {
		return shard, nil
	}

	return nil, fmt.Errorf("invalid shard ID: %d", shardID)
}

// GetShardForData determines the shard responsible for a given key using consistent hashing.
func (sm *ShardManager) GetShardForData(key string) (*Shard, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shardID := sm.hashData(key) % sm.ShardCount
	if shard, exists := sm.Shards[shardID]; exists {
		return shard, nil
	}

	return nil, errors.New("invalid shard ID")
}

// hashValidator hashes the validator's peer ID to determine shard assignment.
func (sm *ShardManager) hashValidator(v peer.ID) int {
	h := fnv.New32a()
	h.Write([]byte(v.String()))
	return int(h.Sum32())
}

// hashData hashes the data key to determine shard assignment.
func (sm *ShardManager) hashData(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32())
}

// RebalanceShards redistributes validators across shards to maintain load balance.
func (sm *ShardManager) RebalanceShards() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Collect all validators
	allValidators := make([]peer.ID, 0)
	for _, shard := range sm.Shards {
		allValidators = append(allValidators, shard.ListValidators()...)
		shard.ClearValidators()
	}

	// Reassign validators
	for _, validatorID := range allValidators {
		shardID := sm.hashValidator(validatorID) % sm.ShardCount
		shard := sm.Shards[shardID]
		shard.AddValidator(validatorID)
	}

	sm.logger.Info("Shards rebalanced")
}

// String returns a string representation of the ShardManager.
func (sm *ShardManager) String() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := "ShardManager State:\n"
	for _, shard := range sm.Shards {
		result += shard.String() + "\n"
	}
	return result
}
