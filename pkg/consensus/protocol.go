// pkg/consensus/protocol.go
package consensus

import (
	"context"
	"fmt"
	"github.com/peerdns/peerdns/pkg/storage"
	"log"
	"sync"
)

// ConsensusProtocol represents the SHPoNU consensus protocol.
type ConsensusProtocol struct {
	state       *ConsensusState       // State management for the consensus
	validators  *ValidatorSet         // Set of validators participating in consensus
	messagePool *ConsensusMessagePool // Pool for storing and managing consensus messages
	logger      *log.Logger           // Logger for protocol events
	storage     *storage.Manager      // Reference to storage manager for state persistence
	mutex       sync.RWMutex          // Mutex for synchronizing consensus operations
	ctx         context.Context       // Context for managing lifecycle
	cancel      context.CancelFunc    // Cancel function to stop the protocol
}

// NewConsensusProtocol creates a new SHPoNU consensus protocol.
func NewConsensusProtocol(ctx context.Context, validators *ValidatorSet, storage *storage.Manager, logger *log.Logger) *ConsensusProtocol {
	childCtx, cancel := context.WithCancel(ctx)
	state := NewConsensusState(storage, logger)
	messagePool := NewConsensusMessagePool(logger)
	return &ConsensusProtocol{
		state:       state,
		validators:  validators,
		messagePool: messagePool,
		storage:     storage,
		logger:      logger,
		ctx:         childCtx,
		cancel:      cancel,
	}
}

// ProposeBlock initiates a new block proposal in the consensus.
func (cp *ConsensusProtocol) ProposeBlock(blockData []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Ensure that only the leader can propose a new block.
	if !cp.validators.IsLeader(cp.validators.CurrentValidator().ID) {
		return ErrNotLeader
	}

	// Create a new proposal message.
	blockHash := HashData(blockData)
	signature, err := SignData(blockHash, cp.validators.CurrentValidator().PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign block: %w", err)
	}

	message := &ConsensusMessage{
		Type:       ProposalMessage,
		BlockData:  blockData,
		BlockHash:  blockHash,
		ProposerID: cp.validators.CurrentValidator().ID,
		Signature:  signature,
	}

	// Add the proposal to the message pool and state.
	cp.messagePool.AddMessage(message)
	cp.state.AddProposal(message)

	cp.logger.Printf("Block proposed by validator %s: %x", message.ProposerID, blockData)

	// Broadcast the proposal message.
	cp.BroadcastMessage(message)
	return nil
}

// ApproveProposal approves a block proposal in the consensus.
func (cp *ConsensusProtocol) ApproveProposal(blockHash []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Ensure that the block is valid and signed correctly.
	proposal := cp.state.GetProposal(blockHash)
	if proposal == nil || !VerifySignature(proposal.BlockHash, proposal.Signature, proposal.ProposerID) {
		return ErrInvalidProposal
	}

	// Create an approval message.
	approvalSignature, err := SignData(blockHash, cp.validators.CurrentValidator().PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign approval: %w", err)
	}

	message := &ConsensusMessage{
		Type:        ApprovalMessage,
		BlockHash:   blockHash,
		ValidatorID: cp.validators.CurrentValidator().ID,
		Signature:   approvalSignature,
	}

	// Add the approval to the message pool and state.
	cp.messagePool.AddMessage(message)
	cp.state.AddApproval(message)

	cp.logger.Printf("Block approved by validator %s: %x", message.ValidatorID, blockHash)

	// Broadcast the approval message.
	cp.BroadcastMessage(message)
	return nil
}

// FinalizeBlock finalizes the block if a quorum of validators have approved it.
func (cp *ConsensusProtocol) FinalizeBlock(blockHash []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Ensure that the block has reached quorum.
	if !cp.state.HasReachedQuorum(blockHash, cp.validators.QuorumSize()) {
		return ErrQuorumNotReached
	}

	// Finalize the block and update storage.
	if err := cp.state.FinalizeBlock(blockHash); err != nil {
		return fmt.Errorf("failed to finalize block: %w", err)
	}

	cp.logger.Printf("Block finalized with hash: %x", blockHash)

	// Broadcast the finalization message.
	finalizationMsg := &ConsensusMessage{
		Type:      FinalizationMessage,
		BlockHash: blockHash,
	}
	cp.BroadcastMessage(finalizationMsg)
	return nil
}

// BroadcastMessage sends a message to all validators in the network.
func (cp *ConsensusProtocol) BroadcastMessage(msg *ConsensusMessage) {
	// Simulate broadcasting the message (replace with actual networking code).
	for _, validator := range cp.validators.GetAllValidators() {
		cp.logger.Printf("Broadcasting message to validator: %s", validator.ID)
		fmt.Printf("Sent message to %s: %v\n", validator.ID, msg)
	}
}

// Shutdown gracefully shuts down the consensus protocol.
func (cp *ConsensusProtocol) Shutdown() {
	cp.cancel()
	cp.logger.Println("Consensus protocol shut down successfully")
}
