package sharding

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestShardManager(t *testing.T) {
	shardCount := 3
	shardMgr := NewShardManager(shardCount)

	// Create dummy peer IDs
	peer1, _ := peer.Decode("QmPeerID1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	peer2, _ := peer.Decode("QmPeerID2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	peer3, _ := peer.Decode("QmPeerID3xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	peer4, _ := peer.Decode("QmPeerID4xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")

	// Assign validators to shards
	err := shardMgr.AssignValidator(peer1)
	require.NoError(t, err, "Failed to assign peer1 to a shard")
	err = shardMgr.AssignValidator(peer2)
	require.NoError(t, err, "Failed to assign peer2 to a shard")
	err = shardMgr.AssignValidator(peer3)
	require.NoError(t, err, "Failed to assign peer3 to a shard")
	err = shardMgr.AssignValidator(peer4)
	require.NoError(t, err, "Failed to assign peer4 to a shard")

	// Ensure validators are assigned to shards
	shardID1, err := shardMgr.GetShardForValidator(peer1)
	require.NoError(t, err, "Failed to get shard for peer1")
	require.True(t, shardMgr.Shards[shardID1].HasValidator(peer1), "Shard %d should have peer1", shardID1)

	shardID2, err := shardMgr.GetShardForValidator(peer2)
	require.NoError(t, err, "Failed to get shard for peer2")
	require.True(t, shardMgr.Shards[shardID2].HasValidator(peer2), "Shard %d should have peer2", shardID2)

	shardID3, err := shardMgr.GetShardForValidator(peer3)
	require.NoError(t, err, "Failed to get shard for peer3")
	require.True(t, shardMgr.Shards[shardID3].HasValidator(peer3), "Shard %d should have peer3", shardID3)

	shardID4, err := shardMgr.GetShardForValidator(peer4)
	require.NoError(t, err, "Failed to get shard for peer4")
	require.True(t, shardMgr.Shards[shardID4].HasValidator(peer4), "Shard %d should have peer4", shardID4)

	// Test GetShardForData
	key := "sample_key"
	shard, err := shardMgr.GetShardForData(key)
	require.NoError(t, err, "Failed to get shard for data key")
	require.NotNil(t, shard, "Shard should not be nil")

	// Test RemoveValidator
	err = shardMgr.RemoveValidator(peer2)
	require.NoError(t, err, "Failed to remove peer2 from shard")
	_, err = shardMgr.GetShardForValidator(peer2)
	require.Error(t, err, "peer2 should no longer be assigned to any shard")
	require.False(t, shardMgr.Shards[shardID2].HasValidator(peer2), "Shard %d should not have peer2", shardID2)
}
