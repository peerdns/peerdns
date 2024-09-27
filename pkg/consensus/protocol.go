package consensus

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
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
func NewConsensusProtocol(ctx context.Context, validators *ValidatorSet, storageMgr *storage.Manager, logger *log.Logger) (*ConsensusProtocol, error) {
	childCtx, cancel := context.WithCancel(ctx)

	// Retrieve or create a specific DB for consensus
	consensusDB, err := storageMgr.GetDb("consensus")
	if err != nil {
		// Optionally, create the DB if it doesn't exist
		consensusDB, err = storageMgr.CreateDb("consensus")
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create consensus DB: %w", err)
		}
	}

	state := NewConsensusState(consensusDB.(*storage.Db), logger)
	messagePool := NewConsensusMessagePool(logger)
	return &ConsensusProtocol{
		state:       state,
		validators:  validators,
		messagePool: messagePool,
		storage:     storageMgr,
		logger:      logger,
		ctx:         childCtx,
		cancel:      cancel,
	}, nil
}

// ProposeBlock initiates a new block proposal in the consensus.
func (cp *ConsensusProtocol) ProposeBlock(blockData []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Ensure that only the leader can propose a new block.
	leader := cp.validators.CurrentLeader()
	if leader == nil || !cp.validators.IsLeader(leader.ID) {
		return ErrNotLeader
	}

	// Create a new proposal message.
	blockHash := HashData(blockData)
	signature, err := SignData(blockHash, leader.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign block: %w", err)
	}

	message := &ConsensusMessage{
		Type:       ProposalMessage,
		BlockData:  blockData,
		BlockHash:  blockHash,
		ProposerID: leader.ID,
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
func (cp *ConsensusProtocol) ApproveProposal(blockHash []byte, approverID peer.ID) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Ensure that the block is valid and signed correctly.
	proposal := cp.state.GetProposal(blockHash)
	if proposal == nil {
		return ErrInvalidProposal
	}

	proposer := cp.validators.GetValidator(proposal.ProposerID)
	if proposer == nil {
		return ErrInvalidProposal
	}

	// Verify the proposal's signature using the proposer's public key
	valid := VerifySignature(proposal.BlockHash, proposal.Signature, proposer.PublicKey)
	if !valid {
		return ErrInvalidSignature
	}

	// Get the approver's private key
	approver := cp.validators.GetValidator(approverID)
	if approver == nil || approver.PrivateKey == nil {
		return fmt.Errorf("approver validator not found or lacks private key")
	}

	// Create an approval message.
	approvalSignature, err := SignData(blockHash, approver.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign approval: %w", err)
	}

	message := &ConsensusMessage{
		Type:        ApprovalMessage,
		BlockHash:   blockHash,
		ValidatorID: approver.ID,
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
