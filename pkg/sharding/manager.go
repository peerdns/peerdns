package sharding

import (
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ShardManager manages all shards in the network.
type ShardManager struct {
	Shards     []*Shard
	ShardCount int
	Validators map[peer.ID]int // Maps validator ID to shard ID
	mu         sync.RWMutex
}

// NewShardManager initializes a new ShardManager with a specified number of shards.
func NewShardManager(shardCount int) *ShardManager {
	shards := make([]*Shard, shardCount)
	for i := 0; i < shardCount; i++ {
		shards[i] = NewShard(i)
	}
	return &ShardManager{
		Shards:     shards,
		ShardCount: shardCount,
		Validators: make(map[peer.ID]int),
	}
}

// AssignValidator assigns a validator to a shard using consistent hashing.
func (sm *ShardManager) AssignValidator(v peer.ID) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.Validators[v]; exists {
		return fmt.Errorf("validator %s is already assigned to shard %d", v, sm.Validators[v])
	}

	shardID := sm.hashValidator(v) % sm.ShardCount
	sm.Validators[v] = shardID
	sm.Shards[shardID].AddValidator(v)
	return nil
}

// RemoveValidator removes a validator from its assigned shard.
func (sm *ShardManager) RemoveValidator(v peer.ID) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	shardID, exists := sm.Validators[v]
	if !exists {
		return fmt.Errorf("validator %s is not assigned to any shard", v)
	}

	sm.Shards[shardID].RemoveValidator(v)
	delete(sm.Validators, v)
	return nil
}

// hashValidator hashes the validator's peer ID to determine shard assignment.
func (sm *ShardManager) hashValidator(v peer.ID) int {
	h := fnv.New32a()
	h.Write([]byte(v.String()))
	return int(h.Sum32())
}

// GetShardForValidator retrieves the shard ID for a given validator.
func (sm *ShardManager) GetShardForValidator(v peer.ID) (int, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shardID, exists := sm.Validators[v]
	if !exists {
		return -1, fmt.Errorf("validator %s is not assigned to any shard", v)
	}
	return shardID, nil
}

// GetShard returns the shard by its ID.
func (sm *ShardManager) GetShard(shardID int) (*Shard, error) {
	if shardID < 0 || shardID >= sm.ShardCount {
		return nil, fmt.Errorf("invalid shard ID: %d", shardID)
	}
	return sm.Shards[shardID], nil
}

// GetShardForData determines the shard responsible for a given key using consistent hashing.
func (sm *ShardManager) GetShardForData(key string) (*Shard, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shardID := sm.hashData(key) % sm.ShardCount
	return sm.Shards[shardID], nil
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

	validatorCount := len(sm.Validators)
	targetPerShard := validatorCount / sm.ShardCount
	if targetPerShard == 0 {
		targetPerShard = 1
	}

	currentShardCounts := make([]int, sm.ShardCount)
	for _, shard := range sm.Shards {
		currentShardCounts[shard.ID] = len(shard.Validators)
	}

	for v, shardID := range sm.Validators {
		if currentShardCounts[shardID] > targetPerShard {
			// Find a shard with less than target
			for _, s := range sm.Shards {
				if len(s.Validators) < targetPerShard {
					// Move validator to this shard
					sm.Shards[shardID].RemoveValidator(v)
					s.AddValidator(v)
					sm.Validators[v] = s.ID
					currentShardCounts[shardID]--
					currentShardCounts[s.ID]++
					break
				}
			}
		}
	}
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
