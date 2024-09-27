package consensus

import (
	"fmt"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/messages"
	"log"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/storage"
)

// ConsensusState manages the state of proposals, approvals, and finalizations.
type ConsensusState struct {
	proposals map[string]*messages.ConsensusMessage           // Map of proposals indexed by block hash
	approvals map[string]map[peer.ID]*encryption.BLSSignature // Map of approvals indexed by block hash and validator ID
	finalized map[string]bool                                 // Map indicating finalized blocks
	storage   *storage.Db                                     // Storage manager for persistent state
	logger    *log.Logger                                     // Logger for state events
	mutex     sync.RWMutex                                    // Mutex for safe access
}

// NewConsensusState creates a new ConsensusState with storage integration.
func NewConsensusState(storage *storage.Db, logger *log.Logger) *ConsensusState {
	return &ConsensusState{
		proposals: make(map[string]*messages.ConsensusMessage),
		approvals: make(map[string]map[peer.ID]*encryption.BLSSignature),
		finalized: make(map[string]bool),
		storage:   storage,
		logger:    logger,
	}
}

// AddProposal adds a new proposal to the state.
func (cs *ConsensusState) AddProposal(msg *messages.ConsensusMessage) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.proposals[string(msg.BlockHash)] = msg

	// Persist proposal to storage
	err := cs.storage.Set(msg.BlockHash, msg.BlockData)
	if err != nil {
		cs.logger.Printf("Failed to persist proposal: %v", err)
	}

	return nil
}

// HasProposal checks if a proposal exists for the given block hash.
func (cs *ConsensusState) HasProposal(blockHash []byte) bool {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	_, exists := cs.proposals[string(blockHash)]
	return exists
}

// GetProposal retrieves a proposal by block hash.
func (cs *ConsensusState) GetProposal(blockHash []byte) *messages.ConsensusMessage {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	return cs.proposals[string(blockHash)]
}

// AddApproval records an approval for a proposal from a validator.
func (cs *ConsensusState) AddApproval(msg *messages.ConsensusMessage) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	if cs.approvals[string(msg.BlockHash)] == nil {
		cs.approvals[string(msg.BlockHash)] = make(map[peer.ID]*encryption.BLSSignature)
	}
	cs.approvals[string(msg.BlockHash)][msg.ValidatorID] = msg.Signature

	// Persist approval to storage
	err := cs.storage.Set(msg.BlockHash, msg.Signature.Signature)
	if err != nil {
		cs.logger.Printf("Failed to persist approval: %v", err)
		return err
	}
	return nil
}

// HasReachedQuorum checks if the proposal has reached quorum for finalization.
func (cs *ConsensusState) HasReachedQuorum(blockHash []byte, quorumSize int) bool {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	approvals := cs.approvals[string(blockHash)]
	return len(approvals) >= quorumSize
}

// FinalizeBlock marks a block as finalized and persists it to storage.
func (cs *ConsensusState) FinalizeBlock(blockHash []byte) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.finalized[string(blockHash)] = true

	// Persist finalization to storage
	err := cs.storage.Set(blockHash, []byte("finalized"))
	if err != nil {
		return fmt.Errorf("failed to persist finalized block: %w", err)
	}
	cs.logger.Printf("Block finalized: %x", blockHash)
	return nil
}

// IsFinalized checks if a block is finalized.
func (cs *ConsensusState) IsFinalized(blockHash []byte) bool {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	return cs.finalized[string(blockHash)]
}
