package sharding

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestShardManager(t *testing.T) {
	// Initialize logger
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel) // Suppress logs during testing
	logger, err := loggerConfig.Build()
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	shardCount := 3
	shardMgr := NewShardManager(shardCount, logger)

	// Create dummy peer IDs
	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")
	peer3 := peer.ID("peer3")
	peer4 := peer.ID("peer4")

	// Assign validators to shards
	shardID1, err := shardMgr.AssignValidator(peer1)
	assert.NoError(t, err, "Failed to assign peer1 to a shard")

	shardID2, err := shardMgr.AssignValidator(peer2)
	assert.NoError(t, err, "Failed to assign peer2 to a shard")

	shardID3, err := shardMgr.AssignValidator(peer3)
	assert.NoError(t, err, "Failed to assign peer3 to a shard")

	shardID4, err := shardMgr.AssignValidator(peer4)
	assert.NoError(t, err, "Failed to assign peer4 to a shard")

	// Ensure validators are assigned to shards
	assert.True(t, shardMgr.Shards[shardID1].HasValidator(peer1), "Shard %d should have peer1", shardID1)
	assert.True(t, shardMgr.Shards[shardID2].HasValidator(peer2), "Shard %d should have peer2", shardID2)
	assert.True(t, shardMgr.Shards[shardID3].HasValidator(peer3), "Shard %d should have peer3", shardID3)
	assert.True(t, shardMgr.Shards[shardID4].HasValidator(peer4), "Shard %d should have peer4", shardID4)

	// Test GetValidatorsInShard
	for shardID := 0; shardID < shardCount; shardID++ {
		validatorsInShard, err := shardMgr.GetValidatorsInShard(shardID)
		assert.NoError(t, err, "Failed to get validators in shard %d", shardID)
		for _, vid := range validatorsInShard {
			shardIDForValidator, err := shardMgr.GetShardForValidator(vid)
			assert.NoError(t, err, "Failed to get shard for validator %s", vid)
			assert.Equal(t, shardID, shardIDForValidator, "Validator %s expected in shard %d, found in shard %d", vid, shardID, shardIDForValidator)
		}
	}

	// Test RemoveValidator
	err = shardMgr.RemoveValidator(peer2)
	assert.NoError(t, err, "Failed to remove peer2 from shard")
	_, err = shardMgr.GetShardForValidator(peer2)
	assert.Error(t, err, "peer2 should no longer be assigned to any shard")
}

func (sm *ShardManager) GetShardForValidator(validatorID peer.ID) (int, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for shardID, shard := range sm.Shards {
		if shard.HasValidator(validatorID) {
			return shardID, nil
		}
	}

	return -1, fmt.Errorf("validator %s is not assigned to any shard", validatorID)
}
