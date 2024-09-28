package sharding

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Shard represents a single shard within the network.
type Shard struct {
	ID         int
	Validators map[peer.ID]bool
	Data       map[string][]byte // Key-value pairs managed by this shard
	mu         sync.RWMutex
}

// NewShard initializes a new Shard with a given ID.
func NewShard(id int) *Shard {
	return &Shard{
		ID:         id,
		Validators: make(map[peer.ID]bool),
		Data:       make(map[string][]byte),
	}
}

// AddValidator adds a validator to the shard.
func (s *Shard) AddValidator(v peer.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Validators[v] = true
}

// RemoveValidator removes a validator from the shard.
func (s *Shard) RemoveValidator(v peer.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Validators, v)
}

// HasValidator checks if a validator is part of the shard.
func (s *Shard) HasValidator(v peer.ID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Validators[v]
}

// StoreData stores a key-value pair in the shard.
func (s *Shard) StoreData(key string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = value
}

// RetrieveData retrieves a value by key from the shard.
func (s *Shard) RetrieveData(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.Data[key]
	return value, exists
}

// ListValidators returns a list of validators in the shard.
func (s *Shard) ListValidators() []peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	validators := make([]peer.ID, 0, len(s.Validators))
	for v := range s.Validators {
		validators = append(validators, v)
	}
	return validators
}

// String returns a string representation of the shard.
func (s *Shard) String() string {
	return fmt.Sprintf("Shard ID: %d, Validators: %v, Data Entries: %d", s.ID, s.ListValidators(), len(s.Data))
}
