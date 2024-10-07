package consensus

import (
	"context"
	"fmt"
	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/peerdns/peerdns/pkg/types"
	"go.uber.org/zap"
	"sync"

	"github.com/pkg/errors"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/packets"
	"github.com/peerdns/peerdns/pkg/storage"
)

// Protocol represents the SHPoNU consensus protocol.
type Protocol struct {
	state           *State                         // State management for the consensus
	validators      *ValidatorSet                  // Set of validators participating in consensus
	logger          logger.Logger                  // Logger for protocol events
	storageMgr      *storage.Manager               // Reference to storage manager for state persistence
	db              *storage.Db                    // Reference to the database instance
	mutex           sync.RWMutex                   // Mutex for synchronizing consensus operations
	approvalMutex   sync.RWMutex                   // Separate mutex for approval operations
	messagePool     *MessagePool                   // Pool for in-flight messages
	ctx             context.Context                // Context for managing lifecycle
	cancel          context.CancelFunc             // Cancel function to stop the protocol
	p2pNetwork      networking.P2PNetworkInterface // Reference to the P2P network for message broadcasting
	blockFinalizer  BlockFinalizer                 // Interface to handle block finalization (optional)
	quorumThreshold int                            // Minimal signing threshold (derived from validators)
	wg              sync.WaitGroup                 // WaitGroup for managing goroutines
}

// NewProtocol creates a new SHPoNU consensus protocol.
// This constructor expects all dependencies to be provided.
func NewProtocol(ctx context.Context, validators *ValidatorSet, storageMgr *storage.Manager, logger logger.Logger, p2pNet networking.P2PNetworkInterface, finalizer BlockFinalizer) (*Protocol, error) {
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
	state := NewState(consensusDB.(*storage.Db), logger)

	// Initialize MessagePool
	messagePool := NewMessagePool(logger)

	// Determine the quorum threshold based on validators
	quorumThreshold := validators.QuorumSize()

	// Initialize the Protocol struct
	cp := &Protocol{
		state:           state,
		validators:      validators,
		logger:          logger,
		storageMgr:      storageMgr,
		db:              consensusDB.(*storage.Db),
		messagePool:     messagePool,
		ctx:             childCtx,
		cancel:          cancel,
		p2pNetwork:      p2pNet,
		blockFinalizer:  finalizer,
		quorumThreshold: quorumThreshold,
	}

	// Start listening to the consensus topic
	if cp.p2pNetwork != nil {
		cp.wg.Add(1)
		go cp.Start()
	}

	return cp, nil
}

// Start begins listening to the consensus topic and processing incoming messages.
func (cp *Protocol) Start() {
	defer cp.wg.Done()

	sub, err := cp.p2pNetwork.PubSubSubscribe("shpounu/1.0.0")
	if err != nil {
		cp.logger.Error("Failed to subscribe to consensus topic", zap.Error(err))
		return
	}
	defer sub.Cancel()

	for {
		select {
		case <-cp.ctx.Done():
			cp.logger.Info("Consensus protocol context cancelled. Exiting Start loop.")
			return
		default:
			msg, err := sub.Next(cp.ctx)
			if err != nil {
				cp.logger.Error("Error receiving message from consensus topic", zap.Error(err))
				return
			}

			if msg == nil {
				// No message received; continue
				continue
			}

			// Deserialize the consensus packet
			consensusPacket, err := packets.DeserializeConsensusPacket(msg.Data)
			if err != nil {
				cp.logger.Error("Failed to deserialize consensus packet", zap.Error(err))
				continue
			}

			// Add the message to the MessagePool
			cp.messagePool.AddMessage(consensusPacket)

			// Process the message asynchronously
			cp.wg.Add(1)
			go func(cp *Protocol, pkt *packets.ConsensusPacket) {
				defer cp.wg.Done()
				cp.HandleMessage(pkt)
				// Remove the message from the pool after processing
				cp.messagePool.RemoveMessage(pkt.BlockHash)
			}(cp, consensusPacket)
		}
	}
}

// Shutdown gracefully shuts down the consensus protocol.
func (cp *Protocol) Shutdown() {
	cp.cancel()
	cp.wg.Wait()
	cp.logger.Info("Consensus protocol shut down successfully")
}

// ProposeBlock allows the leader to propose a new block.
func (cp *Protocol) ProposeBlock(blockData []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Ensure that only the leader can propose a new block.
	leader := cp.validators.CurrentLeader()
	if leader == nil || !cp.validators.IsLeader(leader.PeerID()) {
		return ErrNotLeader
	}

	// Compute block hash
	blockHash := types.HashData(blockData)

	// Sign the block hash using leader's private key
	signature, err := leader.Signer(signatures.BlsSignerType).Sign(blockHash)
	if err != nil {
		return fmt.Errorf("failed to sign block hash: %w", err)
	}

	// Create proposal packet
	proposalPkt := &packets.ConsensusPacket{
		Type:       packets.PacketTypeProposal,
		ProposerID: leader.PeerID(),
		BlockHash:  blockHash,
		BlockData:  blockData,
		Signature:  signature,
	}

	// Add the proposal to the state
	if err := cp.state.AddProposal(proposalPkt); err != nil {
		return errors.Wrap(err, "failed to add proposal")
	}

	cp.logger.Info("Leader proposed block", zap.String("leaderID", leader.PeerID().String()), zap.String("blockHash", fmt.Sprintf("%x", blockHash)))

	// Broadcast the proposal packet
	if err := cp.BroadcastMessage(proposalPkt); err != nil {
		return fmt.Errorf("failed to broadcast proposal packet: %w", err)
	}

	return nil
}

// ApproveProposal allows a validator to approve a block proposal.
func (cp *Protocol) ApproveProposal(blockHash []byte, approverID peer.ID) error {
	cp.approvalMutex.Lock()
	defer cp.approvalMutex.Unlock()

	// Get the approver's validator
	approver := cp.validators.GetValidator(approverID)
	if approver == nil {
		return fmt.Errorf("approver validator not found")
	}

	// Sign the block hash
	signature, err := approver.Signer(signatures.BlsSignerType).Sign(blockHash)
	if err != nil {
		return fmt.Errorf("failed to sign block hash: %w", err)
	}

	// Create approval packet
	approvalPkt := &packets.ConsensusPacket{
		Type:        packets.PacketTypeApproval,
		ValidatorID: approver.PeerID(),
		BlockHash:   blockHash,
		Signature:   signature,
	}

	// Add the approval to the state
	if err := cp.state.AddApproval(approvalPkt); err != nil {
		return fmt.Errorf("failed to add approval packet: %w", err)
	}

	cp.logger.Info("Validator approved block", zap.String("validatorID", approver.PeerID().String()), zap.String("blockHash", fmt.Sprintf("%x", blockHash)))

	// Broadcast the approval packet
	if err := cp.BroadcastMessage(approvalPkt); err != nil {
		return fmt.Errorf("failed to broadcast approval packet: %w", err)
	}

	// Check if quorum is reached
	if cp.state.HasReachedQuorum(blockHash, cp.quorumThreshold) {
		if err := cp.FinalizeBlock(blockHash); err != nil {
			cp.logger.Error("Failed to finalize block", zap.Error(err))
		}
	}

	return nil
}

// FinalizeBlock finalizes a block if quorum is reached.
func (cp *Protocol) FinalizeBlock(blockHash []byte) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Check if the block is already finalized
	if cp.state.IsFinalized(blockHash) {
		cp.logger.Info("Block is already finalized", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))
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

	cp.logger.Info("Block finalized", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))

	// Optionally, invoke the block finalizer
	if cp.blockFinalizer != nil {
		blockData, err := cp.db.Get(blockHash)
		if err != nil {
			cp.logger.Error("Failed to retrieve block data for finalization", zap.Error(err))
			return err
		}
		if err := cp.blockFinalizer.Finalize(blockHash, blockData); err != nil {
			cp.logger.Error("Block finalizer failed", zap.Error(err))
			return err
		}
	}

	// Create a FinalizationPacket without a signature
	finalizationPkt := &packets.ConsensusPacket{
		Type:      packets.PacketTypeFinalization,
		BlockHash: blockHash,
		// Signature is intentionally left as nil
	}

	// Broadcast the finalization packet
	if err := cp.BroadcastMessage(finalizationPkt); err != nil {
		return fmt.Errorf("failed to broadcast finalization packet: %w", err)
	}

	return nil
}

// HandleMessage processes incoming consensus packets.
func (cp *Protocol) HandleMessage(pkt *packets.ConsensusPacket) {
	switch pkt.Type {
	case packets.PacketTypeProposal:
		cp.HandleProposal(pkt)
	case packets.PacketTypeApproval:
		cp.HandleApproval(pkt)
	case packets.PacketTypeFinalization:
		cp.HandleFinalization(pkt)
	default:
		cp.logger.Warn("Received unknown packet type", zap.String("type", fmt.Sprintf("%v", pkt.Type)))
	}
}

// HandleProposal processes a block proposal.
func (cp *Protocol) HandleProposal(pkt *packets.ConsensusPacket) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Check if the proposal already exists
	if cp.state.HasProposal(pkt.BlockHash) {
		cp.logger.Info("Proposal already exists", zap.String("blockHash", fmt.Sprintf("%x", pkt.BlockHash)))
		return
	}

	// Verify the proposal's signature
	proposer := cp.validators.GetValidator(pkt.ProposerID)
	if proposer == nil {
		cp.logger.Warn("Unknown proposer", zap.String("proposerID", pkt.ProposerID.String()))
		return
	}

	if valid, vErr := proposer.Signer(signatures.BlsSignerType).Verify(pkt.BlockHash, pkt.Signature); !valid {
		cp.logger.Warn("Invalid signature from proposer", zap.String("proposerID", pkt.ProposerID.String()), zap.Error(vErr))
		return
	}

	// Add the proposal to the state
	if err := cp.state.AddProposal(pkt); err != nil {
		cp.logger.Error("Failed to add proposal", zap.Error(err))
		return
	}

	cp.logger.Info("Received proposal", zap.String("blockHash", fmt.Sprintf("%x", pkt.BlockHash)), zap.String("proposerID", pkt.ProposerID.String()))

	// Auto-approve if this node is a validator and not the proposer
	nodeID := cp.p2pNetwork.HostID()
	if cp.validators.IsValidator(nodeID) && nodeID != pkt.ProposerID {
		cp.logger.Info("Auto-approving block", zap.String("blockHash", fmt.Sprintf("%x", pkt.BlockHash)), zap.String("validatorID", nodeID.String()))
		err := cp.ApproveProposal(pkt.BlockHash, nodeID)
		if err != nil {
			cp.logger.Error("Failed to auto-approve block", zap.Error(err))
		} else {
			cp.logger.Info("Auto-approved block", zap.String("blockHash", fmt.Sprintf("%x", pkt.BlockHash)))
		}
	}
}

// HandleApproval processes a block approval.
func (cp *Protocol) HandleApproval(pkt *packets.ConsensusPacket) {
	cp.approvalMutex.Lock()
	defer cp.approvalMutex.Unlock()

	// Verify the approval's signature
	approver := cp.validators.GetValidator(pkt.ValidatorID)
	if approver == nil {
		cp.logger.Warn("Unknown approver", zap.String("approverID", pkt.ValidatorID.String()))
		return
	}

	if valid, vErr := approver.Signer(signatures.BlsSignerType).Verify(pkt.BlockHash, pkt.Signature); !valid {
		cp.logger.Error("Invalid signature from approver", zap.String("approverID", pkt.ValidatorID.String()), zap.Error(vErr))
		return
	}

	// Add the approval to the state
	if err := cp.state.AddApproval(pkt); err != nil {
		cp.logger.Error("Failed to add approval packet", zap.Error(err))
		return
	}

	cp.logger.Info("Received approval", zap.String("blockHash", fmt.Sprintf("%x", pkt.BlockHash)), zap.String("approverID", pkt.ValidatorID.String()))

	// Check if the block has reached the threshold
	if cp.state.HasReachedQuorum(pkt.BlockHash, cp.quorumThreshold) {
		// Finalize the block
		if err := cp.FinalizeBlock(pkt.BlockHash); err != nil {
			cp.logger.Error("Failed to finalize block", zap.Error(err))
		}
	}
}

// HandleFinalization processes a block finalization.
func (cp *Protocol) HandleFinalization(pkt *packets.ConsensusPacket) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	blockHashKey := fmt.Sprintf("%x", pkt.BlockHash)

	// Check if the block is already finalized
	if cp.state.IsFinalized(pkt.BlockHash) {
		cp.logger.Info("Block is already finalized", zap.String("blockHash", blockHashKey))
		return
	}

	// Finalize the block in the state
	if err := cp.state.FinalizeBlock(pkt.BlockHash); err != nil {
		cp.logger.Error("Failed to finalize block", zap.Error(err))
		return
	}

	cp.logger.Info("Block finalized", zap.String("blockHash", blockHashKey))

	// Optionally, invoke the block finalizer
	if cp.blockFinalizer != nil {
		blockData, err := cp.db.Get(pkt.BlockHash)
		if err != nil {
			cp.logger.Error("Failed to retrieve block data for finalization", zap.Error(err))
			return
		}
		if err := cp.blockFinalizer.Finalize(pkt.BlockHash, blockData); err != nil {
			cp.logger.Error("Block finalizer failed", zap.Error(err))
			return
		}
	}

	// Create a FinalizationPacket without a signature
	finalizationPkt := &packets.ConsensusPacket{
		Type:      packets.PacketTypeFinalization,
		BlockHash: pkt.BlockHash,
		// Signature is intentionally left as nil
	}

	// Broadcast the finalization packet
	if err := cp.BroadcastMessage(finalizationPkt); err != nil {
		cp.logger.Error("Failed to broadcast finalization packet", zap.Error(err))
	}
}

// BroadcastMessage publishes a ConsensusPacket to the network.
func (cp *Protocol) BroadcastMessage(pkt *packets.ConsensusPacket) error {
	if pkt == nil {
		return fmt.Errorf("consensus packet is nil")
	}
	serializedPkt, err := pkt.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize consensus packet: %w", err)
	}
	if cp.p2pNetwork == nil {
		// In tests, p2pNetwork might be nil. Skip broadcasting.
		return nil
	}
	return cp.p2pNetwork.BroadcastMessage(serializedPkt)
}
