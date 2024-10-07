package consensus

import "sync"

// MockBlockFinalizer is a mock implementation for block finalization.
type MockBlockFinalizer struct {
	finalizedBlocks map[string][]byte
	mu              sync.Mutex
}

// NewMockBlockFinalizer initializes a new MockBlockFinalizer.
func NewMockBlockFinalizer() *MockBlockFinalizer {
	return &MockBlockFinalizer{
		finalizedBlocks: make(map[string][]byte),
	}
}

// Finalize finalizes a block.
func (bf *MockBlockFinalizer) Finalize(blockHash []byte, blockData []byte) error {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.finalizedBlocks[string(blockHash)] = blockData
	return nil
}

// IsFinalized checks if a block is finalized.
func (bf *MockBlockFinalizer) IsFinalized(blockHash []byte) bool {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	_, exists := bf.finalizedBlocks[string(blockHash)]
	return exists
}
