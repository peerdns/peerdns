// pkg/consensus/consensus_state.go
package consensus

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/messages"
	"github.com/peerdns/peerdns/pkg/storage"
	"go.uber.org/zap"
)

// ConsensusState manages the state of proposals, approvals, and finalizations.
type ConsensusState struct {
	proposalsMu sync.RWMutex
	proposals   map[[32]byte]*messages.ConsensusMessage // Using fixed-size array as key

	approvalsMu sync.RWMutex
	approvals   map[[32]byte]map[peer.ID]*encryption.BLSSignature // Using fixed-size array as key

	finalizedMu sync.RWMutex
	finalized   map[[32]byte]bool // Using fixed-size array as key

	storage *storage.Db
	logger  logger.Logger
}

// NewConsensusState creates a new ConsensusState with storage integration.
func NewConsensusState(storage *storage.Db, logger logger.Logger) *ConsensusState {
	return &ConsensusState{
		proposals: make(map[[32]byte]*messages.ConsensusMessage, 1000),
		approvals: make(map[[32]byte]map[peer.ID]*encryption.BLSSignature, 1000),
		finalized: make(map[[32]byte]bool, 1000),
		storage:   storage,
		logger:    logger,
	}
}

// AddProposal adds a new proposal to the state.
func (cs *ConsensusState) AddProposal(msg *messages.ConsensusMessage) error {
	var blockHash [32]byte
	copy(blockHash[:], msg.BlockHash)

	// Update in-memory proposals
	cs.proposalsMu.Lock()
	cs.proposals[blockHash] = msg
	cs.proposalsMu.Unlock()

	// Persist proposal to storage
	err := cs.storage.Set(msg.BlockHash, msg.BlockData)
	if err != nil {
		cs.logger.Error(
			"Failed to persist proposal",
			zap.Error(err),
			zap.String("blockHash", fmt.Sprintf("%x", msg.BlockHash)),
		)
		return err
	}

	return nil
}

// HasProposal checks if a proposal exists for the given block hash.
func (cs *ConsensusState) HasProposal(blockHash []byte) bool {
	var bh [32]byte
	copy(bh[:], blockHash)

	cs.proposalsMu.RLock()
	_, exists := cs.proposals[bh]
	cs.proposalsMu.RUnlock()

	return exists
}

// AddApproval records an approval for a proposal from a validator.
func (cs *ConsensusState) AddApproval(msg *messages.ConsensusMessage) error {
	var blockHash [32]byte
	copy(blockHash[:], msg.BlockHash)

	// Update in-memory approvals
	cs.approvalsMu.Lock()
	approvalsForBlock, exists := cs.approvals[blockHash]
	if !exists {
		approvalsForBlock = make(map[peer.ID]*encryption.BLSSignature)
		cs.approvals[blockHash] = approvalsForBlock
	}

	// Check if this validator has already approved
	if _, approved := approvalsForBlock[msg.ValidatorID]; approved {
		cs.approvalsMu.Unlock()
		cs.logger.Info(
			"Validator has already approved block",
			zap.String("validator", string(msg.ValidatorID)),
			zap.String("blockHash", fmt.Sprintf("%x", msg.BlockHash)),
		)
		return nil
	}

	approvalsForBlock[msg.ValidatorID] = msg.Signature
	approvalCount := len(approvalsForBlock)
	cs.approvalsMu.Unlock()

	// Log the current number of approvals for this block hash
	cs.logger.Info(
		"Approval added. Total approvals for block",
		zap.String("blockHash", fmt.Sprintf("%x", msg.BlockHash)),
		zap.Int("approvalCount", approvalCount),
	)

	cs.logger.Info(
		"Current approval map for block",
		zap.String("blockHash", fmt.Sprintf("%x", msg.BlockHash)),
		zap.Any("approvals", approvalsForBlock),
	)

	// Persist approval to storage (outside the lock)
	err := cs.storage.Set(msg.BlockHash, msg.Signature.Signature)
	if err != nil {
		cs.logger.Error(
			"Failed to persist approval",
			zap.Error(err),
			zap.String("blockHash", fmt.Sprintf("%x", msg.BlockHash)),
			zap.String("validatorID", string(msg.ValidatorID)),
		)
		return err
	}

	return nil
}

// HasReachedQuorum checks if the proposal has reached quorum for finalization.
func (cs *ConsensusState) HasReachedQuorum(blockHash []byte, quorumSize int) bool {
	var bh [32]byte
	copy(bh[:], blockHash)

	cs.approvalsMu.RLock()
	approvals := cs.approvals[bh]
	approvalCount := len(approvals)
	cs.approvalsMu.RUnlock()

	cs.logger.Info(
		"Quorum check for block",
		zap.String("blockHash", fmt.Sprintf("%x", blockHash)),
		zap.Int("approvals", approvalCount),
		zap.Int("quorumSize", quorumSize),
	)
	return approvalCount >= quorumSize
}

// FinalizeBlock marks a block as finalized and persists it to storage.
func (cs *ConsensusState) FinalizeBlock(blockHash []byte) error {
	var bh [32]byte
	copy(bh[:], blockHash)

	// Update in-memory finalized map
	cs.finalizedMu.Lock()
	cs.finalized[bh] = true
	cs.finalizedMu.Unlock()

	// Persist finalization to storage
	err := cs.storage.Set(blockHash, []byte("finalized"))
	if err != nil {
		cs.logger.Error(
			"Failed to persist finalized block",
			zap.Error(err),
			zap.String("blockHash", fmt.Sprintf("%x", blockHash)),
		)
		return fmt.Errorf("failed to persist finalized block: %w", err)
	}
	cs.logger.Info(
		"Block finalized",
		zap.String("blockHash", fmt.Sprintf("%x", blockHash)),
	)
	return nil
}

// IsFinalized checks if a block is finalized.
func (cs *ConsensusState) IsFinalized(blockHash []byte) bool {
	var bh [32]byte
	copy(bh[:], blockHash)

	cs.finalizedMu.RLock()
	finalized := cs.finalized[bh]
	cs.finalizedMu.RUnlock()

	return finalized
}

// GetApprovalCount returns the number of approvals for a specific block hash.
func (cs *ConsensusState) GetApprovalCount(blockHash []byte) int {
	var bh [32]byte
	copy(bh[:], blockHash)

	cs.approvalsMu.RLock()
	count := len(cs.approvals[bh])
	cs.approvalsMu.RUnlock()

	return count
}
