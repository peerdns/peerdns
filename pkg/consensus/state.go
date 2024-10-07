package consensus

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/packets"
	"github.com/peerdns/peerdns/pkg/storage"
	"go.uber.org/zap"
)

// State manages the state of proposals, approvals, and finalizations.
type State struct {
	proposalsMu sync.RWMutex
	proposals   map[string]*packets.ConsensusPacket // Map of messages indexed by block hash

	approvalsMu sync.RWMutex
	approvals   map[string]map[peer.ID][]byte // Map of block hash to validator approvals

	finalizedMu sync.RWMutex
	finalized   map[string]bool // Map of block hash to finalization status

	storage *storage.Db
	logger  logger.Logger
}

// NewState creates a new ConsensusState.
func NewState(storage *storage.Db, logger logger.Logger) *State {
	return &State{
		proposals: make(map[string]*packets.ConsensusPacket, 1000),
		approvals: make(map[string]map[peer.ID][]byte, 1000),
		finalized: make(map[string]bool, 1000),
		storage:   storage,
		logger:    logger,
	}
}

// AddProposal adds a new proposal to the state.
func (cs *State) AddProposal(msg *packets.ConsensusPacket) error {
	if msg.Type != packets.PacketTypeProposal {
		return fmt.Errorf("invalid packet type for proposal: %v", msg.Type)
	}

	blockHashKey := fmt.Sprintf("%x", msg.BlockHash)

	// Update in-memory proposals
	cs.proposalsMu.Lock()
	cs.proposals[blockHashKey] = msg
	cs.proposalsMu.Unlock()

	// Persist proposal to storage
	err := cs.storage.Set(msg.BlockHash, msg.BlockData)
	if err != nil {
		cs.logger.Error(
			"Failed to persist proposal",
			zap.Error(err),
			zap.String("blockHash", blockHashKey),
		)
		return err
	}

	return nil
}

// HasProposal checks if a proposal exists for the given block hash.
func (cs *State) HasProposal(blockHash []byte) bool {
	blockHashKey := fmt.Sprintf("%x", blockHash)

	cs.proposalsMu.RLock()
	_, exists := cs.proposals[blockHashKey]
	cs.proposalsMu.RUnlock()

	return exists
}

// AddApproval records an approval for a proposal from a validator.
func (cs *State) AddApproval(msg *packets.ConsensusPacket) error {
	if msg.Type != packets.PacketTypeApproval {
		return fmt.Errorf("invalid packet type for approval: %v", msg.Type)
	}

	blockHashKey := fmt.Sprintf("%x", msg.BlockHash)

	// Update in-memory approvals
	cs.approvalsMu.Lock()
	approvalsForBlock, exists := cs.approvals[blockHashKey]
	if !exists {
		approvalsForBlock = make(map[peer.ID][]byte)
		cs.approvals[blockHashKey] = approvalsForBlock
	}

	// Check if this validator has already approved
	if _, approved := approvalsForBlock[msg.ValidatorID]; approved {
		cs.approvalsMu.Unlock()
		cs.logger.Info(
			"Validator has already approved block",
			zap.String("validator", msg.ValidatorID.String()),
			zap.String("blockHash", blockHashKey),
		)
		return nil
	}

	approvalsForBlock[msg.ValidatorID] = msg.Signature
	approvalCount := len(approvalsForBlock)
	cs.approvalsMu.Unlock()

	// Log the current number of approvals for this block hash
	cs.logger.Info(
		"Approval added. Total approvals for block",
		zap.String("blockHash", blockHashKey),
		zap.Int("approvalCount", approvalCount),
	)

	cs.logger.Info(
		"Current approval map for block",
		zap.String("blockHash", blockHashKey),
		zap.Any("approvals", approvalsForBlock),
	)

	// Persist approval to storage (outside the lock)
	err := cs.storage.Set(msg.BlockHash, msg.Signature)
	if err != nil {
		cs.logger.Error(
			"Failed to persist approval",
			zap.Error(err),
			zap.String("blockHash", blockHashKey),
			zap.String("validatorID", msg.ValidatorID.String()),
		)
		return err
	}

	return nil
}

// HasReachedQuorum checks if the proposal has reached quorum for finalization.
func (cs *State) HasReachedQuorum(blockHash []byte, quorumSize int) bool {
	blockHashKey := fmt.Sprintf("%x", blockHash)

	cs.approvalsMu.RLock()
	approvals := cs.approvals[blockHashKey]
	approvalCount := len(approvals)
	cs.approvalsMu.RUnlock()

	cs.logger.Info(
		"Quorum check for block",
		zap.String("blockHash", blockHashKey),
		zap.Int("approvals", approvalCount),
		zap.Int("quorumSize", quorumSize),
	)
	return approvalCount >= quorumSize
}

// FinalizeBlock marks a block as finalized and persists it to storage.
func (cs *State) FinalizeBlock(blockHash []byte) error {
	blockHashKey := fmt.Sprintf("%x", blockHash)

	// Update in-memory finalized map
	cs.finalizedMu.Lock()
	cs.finalized[blockHashKey] = true
	cs.finalizedMu.Unlock()

	// Persist finalization to storage
	err := cs.storage.Set(blockHash, []byte("finalized"))
	if err != nil {
		cs.logger.Error(
			"Failed to persist finalized block",
			zap.Error(err),
			zap.String("blockHash", blockHashKey),
		)
		return fmt.Errorf("failed to persist finalized block: %w", err)
	}
	cs.logger.Info(
		"Block finalized",
		zap.String("blockHash", blockHashKey),
	)
	return nil
}

// IsFinalized checks if a block is finalized.
func (cs *State) IsFinalized(blockHash []byte) bool {
	blockHashKey := fmt.Sprintf("%x", blockHash)

	cs.finalizedMu.RLock()
	finalized := cs.finalized[blockHashKey]
	cs.finalizedMu.RUnlock()

	return finalized
}

// GetApprovalCount returns the number of approvals for a specific block hash.
func (cs *State) GetApprovalCount(blockHash []byte) int {
	blockHashKey := fmt.Sprintf("%x", blockHash)

	cs.approvalsMu.RLock()
	count := len(cs.approvals[blockHashKey])
	cs.approvalsMu.RUnlock()

	return count
}
