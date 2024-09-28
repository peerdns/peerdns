package consensus

import (
	"context"
	"fmt"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/messages"
	"github.com/pkg/errors"
	"log"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/storage"
)

// Protocol represents the SHPoNU consensus protocol.
type Protocol struct {
	state           *ConsensusState                // State management for the consensus
	validators      *ValidatorSet                  // Set of validators participating in consensus
	logger          *log.Logger                    // Logger for protocol events
	storageMgr      *storage.Manager               // Reference to storage manager for state persistence
	db              *storage.Db                    // Reference to the database instance
	mutex           sync.RWMutex                   // Mutex for synchronizing consensus operations
	ctx             context.Context                // Context for managing lifecycle
	cancel          context.CancelFunc             // Cancel function to stop the protocol
	p2pNetwork      networking.P2PNetworkInterface // Reference to the P2P network for message broadcasting
	blockFinalizer  BlockFinalizer                 // Interface to handle block finalization (optional)
	quorumThreshold int                            // Minimal signing threshold (derived from validators)
}

// NewProtocol creates a new SHPoNU consensus protocol.
// This constructor expects all dependencies to be provided.
func NewProtocol(ctx context.Context, validators *ValidatorSet, storageMgr *storage.Manager, logger *log.Logger, p2pNet networking.P2PNetworkInterface, finalizer BlockFinalizer) (*Protocol, error) {
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

	// Initialize ConsensusState with consensusDB
	state := NewConsensusState(consensusDB.(*storage.Db), logger)

	// Determine the quorum threshold based on validators
	quorumThreshold := validators.QuorumSize()

	// Initialize the ConsensusProtocol struct
	cp := &Protocol{
		state:           state,
		validators:      validators,
		logger:          logger,
		storageMgr:      storageMgr,
		db:              consensusDB.(*storage.Db),
		ctx:             childCtx,
		cancel:          cancel,
		p2pNetwork:      p2pNet,
		blockFinalizer:  finalizer,
		quorumThreshold: quorumThreshold,
	}

	// Start listening to the consensus topic
	if cp.p2pNetwork != nil {
		go cp.Start()
	}

	return cp, nil
}

// Start begins listening to the consensus topic and processing incoming messages.
func (cp *Protocol) Start() {
	sub, err := cp.p2pNetwork.PubSubSubscribe("shpounu/1.0.0")
	if err != nil {
		cp.logger.Printf("Failed to subscribe to consensus topic: %v", err)
		return
	}
	defer sub.Cancel()

	for {
		msg, err := sub.Next(cp.ctx)
		if err != nil {
			cp.logger.Printf("Error receiving message from consensus topic: %v", err)
			return
		}

		if msg == nil {
			// No message received; continue
			continue
		}

		// Deserialize the consensus message
		consensusMsg, err := messages.DeserializeConsensusMessage(msg.Data)
		if err != nil {
			cp.logger.Printf("Failed to deserialize consensus message: %v", err)
			continue
		}

		// Handle the consensus message
		cp.HandleMessage(consensusMsg)
	}
}

// Shutdown gracefully shuts down the consensus protocol.
func (cp *Protocol) Shutdown() {
	cp.cancel()
	cp.logger.Println("Consensus protocol shut down successfully")
}

// ProposeBlock allows the leader to propose a new block.
func (cp *Protocol) ProposeBlock(blockData []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Ensure that only the leader can propose a new block.
	leader := cp.validators.CurrentLeader()
	if leader == nil || !cp.validators.IsLeader(leader.ID) {
		return ErrNotLeader
	}

	// Compute block hash
	blockHash := HashData(blockData)

	// Sign the block hash using leader's private key
	signature, err := encryption.Sign(blockHash, leader.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign block hash: %w", err)
	}

	// Create proposal message
	proposalMsg := &messages.ConsensusMessage{
		Type:       messages.ProposalMessage,
		BlockHash:  blockHash,
		ProposerID: leader.ID,
		BlockData:  blockData,
		Signature:  signature,
	}

	// Add the proposal to the state
	if err := cp.state.AddProposal(proposalMsg); err != nil {
		return errors.Wrap(err, "failed to add proposal")
	}

	cp.logger.Printf("Leader %s proposed block: %x", leader.ID.String(), blockHash)

	// Broadcast the proposal message
	if err := cp.BroadcastMessage(proposalMsg); err != nil {
		return fmt.Errorf("failed to broadcast proposal message: %w", err)
	}

	return nil
}

// ApproveProposal allows a validator to approve a block proposal.
func (cp *Protocol) ApproveProposal(blockHash []byte, approverID peer.ID) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Get the approver's validator
	approver := cp.validators.GetValidator(approverID)
	if approver == nil {
		return fmt.Errorf("approver validator not found")
	}

	// Sign the block hash
	signature, err := encryption.Sign(blockHash, approver.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign block hash: %w", err)
	}

	// Create approval message
	approvalMsg := &messages.ConsensusMessage{
		Type:        messages.ApprovalMessage,
		BlockHash:   blockHash,
		ValidatorID: approver.ID,
		Signature:   signature,
	}

	// Add the approval to the state
	if err := cp.state.AddApproval(approvalMsg); err != nil {
		return fmt.Errorf("failed to add approval message: %w", err)
	}

	cp.logger.Printf("Validator %s approved block: %x", approver.ID.String(), blockHash)

	// Broadcast the approval message
	if err := cp.BroadcastMessage(approvalMsg); err != nil {
		return fmt.Errorf("failed to broadcast approval message: %w", err)
	}

	return nil
}

// FinalizeBlock finalizes a block if quorum is reached.
func (cp *Protocol) FinalizeBlock(blockHash []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Check if the block has already been finalized
	if cp.state.IsFinalized(blockHash) {
		cp.logger.Printf("Block %x is already finalized.", blockHash)
		return nil
	}

	// Check if the block has reached the quorum threshold
	if !cp.state.HasReachedQuorum(blockHash, cp.quorumThreshold) {
		return ErrQuorumNotReached
	}

	// Finalize the block in the state
	if err := cp.state.FinalizeBlock(blockHash); err != nil {
		return fmt.Errorf("failed to finalize block: %w", err)
	}

	cp.logger.Printf("Block finalized: %x", blockHash)

	// Optionally, invoke the block finalizer
	if cp.blockFinalizer != nil {
		blockData, err := cp.db.Get(blockHash)
		if err != nil {
			cp.logger.Printf("Failed to retrieve block data for finalization: %v", err)
			return err
		}
		if err := cp.blockFinalizer.Finalize(blockHash, blockData); err != nil {
			cp.logger.Printf("Block finalizer failed: %v", err)
			return err
		}
	}

	// Create a FinalizationMessage without a signature
	finalizationMsg := &messages.ConsensusMessage{
		Type:      messages.FinalizationMessage,
		BlockHash: blockHash,
		// Signature is intentionally left as nil
	}

	// Broadcast the finalization message
	if err := cp.BroadcastMessage(finalizationMsg); err != nil {
		return fmt.Errorf("failed to broadcast finalization message: %w", err)
	}

	return nil
}

// HandleMessage processes incoming consensus messages.
func (cp *Protocol) HandleMessage(msg *messages.ConsensusMessage) {
	switch msg.Type {
	case messages.ProposalMessage:
		cp.HandleProposal(msg)
	case messages.ApprovalMessage:
		cp.HandleApproval(msg)
	case messages.FinalizationMessage:
		cp.HandleFinalization(msg)
	default:
		cp.logger.Printf("Received unknown message type: %v", msg.Type)
	}
}

// HandleProposal processes a block proposal.
func (cp *Protocol) HandleProposal(msg *messages.ConsensusMessage) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Check if the proposal already exists
	if cp.state.HasProposal(msg.BlockHash) {
		cp.logger.Printf("Proposal for block %x already exists.", msg.BlockHash)
		return
	}

	// Verify the proposal's signature
	proposer := cp.validators.GetValidator(msg.ProposerID)
	if proposer == nil {
		cp.logger.Printf("Unknown proposer: %s", msg.ProposerID.String())
		return
	}

	if !encryption.Verify(msg.BlockHash, msg.Signature, proposer.PublicKey) {
		cp.logger.Printf("Invalid signature from proposer: %s", msg.ProposerID.String())
		return
	}

	// Add the proposal to the state
	if err := cp.state.AddProposal(msg); err != nil {
		cp.logger.Printf("Failed to add proposal: %v", err)
		return
	}

	cp.logger.Printf("Received proposal for block: %x from %s", msg.BlockHash, msg.ProposerID.String())

	// Auto-approve if this node is a validator
	if cp.validators.IsValidator(cp.p2pNetwork.HostID()) {
		cp.logger.Printf("Auto-approving block: %x by %s", msg.BlockHash, cp.p2pNetwork.HostID())
		err := cp.ApproveProposal(msg.BlockHash, cp.p2pNetwork.HostID())
		if err != nil {
			cp.logger.Printf("Failed to auto-approve block %x: %v", msg.BlockHash, err)
		} else {
			cp.logger.Printf("Auto-approved block: %x", msg.BlockHash)
		}
	}

	// Automatically approve the proposal from this node if it's not the proposer
	nodeID := cp.p2pNetwork.HostID()
	if nodeID != msg.ProposerID {
		if err := cp.ApproveProposal(msg.BlockHash, nodeID); err != nil {
			cp.logger.Printf("Failed to approve block: %v", err)
		}
	}
}

// HandleApproval processes a block approval.
func (cp *Protocol) HandleApproval(msg *messages.ConsensusMessage) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Verify the approval's signature
	approver := cp.validators.GetValidator(msg.ValidatorID)
	if approver == nil {
		cp.logger.Printf("Unknown approver: %s", msg.ValidatorID.String())
		return
	}

	if !encryption.Verify(msg.BlockHash, msg.Signature, approver.PublicKey) {
		cp.logger.Printf("Invalid signature from approver: %s", msg.ValidatorID.String())
		return
	}

	// Add the approval to the state
	if err := cp.state.AddApproval(msg); err != nil {
		cp.logger.Printf("Failed to add approval: %v", err)
		return
	}

	cp.logger.Printf("Received approval for block: %x from %s", msg.BlockHash, msg.ValidatorID.String())

	// Check if the block has reached the threshold
	if cp.state.HasReachedQuorum(msg.BlockHash, cp.quorumThreshold) {
		// Finalize the block
		if err := cp.FinalizeBlock(msg.BlockHash); err != nil {
			cp.logger.Printf("Failed to finalize block: %v", err)
		}
	}
}

// HandleFinalization processes a block finalization.
func (cp *Protocol) HandleFinalization(msg *messages.ConsensusMessage) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Check if the block is already finalized
	if cp.state.IsFinalized(msg.BlockHash) {
		cp.logger.Printf("Block %x is already finalized. Ignoring finalization message.", msg.BlockHash)
		return
	}

	// Finalize the block in the state
	if err := cp.state.FinalizeBlock(msg.BlockHash); err != nil {
		cp.logger.Printf("Failed to finalize block: %v", err)
		return
	}

	cp.logger.Printf("Block finalized: %x", msg.BlockHash)

	// Optionally, invoke the block finalizer
	if cp.blockFinalizer != nil {
		blockData, err := cp.db.Get(msg.BlockHash)
		if err != nil {
			cp.logger.Printf("Failed to retrieve block data for finalization: %v", err)
			return
		}
		if err := cp.blockFinalizer.Finalize(msg.BlockHash, blockData); err != nil {
			cp.logger.Printf("Block finalizer failed: %v", err)
			return
		}
	}

	// Create a FinalizationMessage without a signature
	finalizationMsg := &messages.ConsensusMessage{
		Type:      messages.FinalizationMessage,
		BlockHash: msg.BlockHash,
		// Signature is intentionally left as nil
	}

	// Broadcast the finalization message
	if err := cp.BroadcastMessage(finalizationMsg); err != nil {
		cp.logger.Printf("Failed to broadcast finalization message: %v", err)
	}
}

// BroadcastMessage publishes a ConsensusMessage to the network.
func (cp *Protocol) BroadcastMessage(msg *messages.ConsensusMessage) error {
	if msg == nil {
		return fmt.Errorf("consensus message is nil")
	}
	serializedMsg, err := msg.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize consensus message: %w", err)
	}
	if cp.p2pNetwork == nil {
		// In tests, p2pNetwork might be nil. Skip broadcasting.
		return nil
	}
	return cp.p2pNetwork.BroadcastMessage(serializedMsg)
}
