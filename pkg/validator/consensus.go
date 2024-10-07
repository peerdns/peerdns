package validator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/metrics"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/packets"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/peerdns/peerdns/pkg/storage"
	"go.uber.org/zap"
)

// Consensus represents the consensus module of the node, managing consensus-related activities.
type Consensus struct {
	identity      *accounts.DID
	network       *networking.Network
	shardManager  *sharding.ShardManager
	storage       *storage.Db
	logger        logger.Logger
	state         *consensus.ConsensusState
	stateMgr      *StateManager
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	messageChan   chan *packets.ConsensusPacket
	processedMsgs map[string]bool // To prevent processing duplicate messages
	validatorSet  *consensus.ValidatorSet
	currentLeader peer.ID
	collector     *metrics.Collector
}

// NewConsensus creates and initializes a new Consensus module.
func NewConsensus(ctx context.Context, did *accounts.DID, network *networking.Network, shardMgr *sharding.ShardManager, store *storage.Db, logger logger.Logger, validatorSet *consensus.ValidatorSet, collector *metrics.Collector, stateMgr *StateManager) *Consensus {
	state := consensus.NewConsensusState(store, logger)
	moduleCtx, cancel := context.WithCancel(ctx)

	// Initialize the Consensus module
	consensusModule := &Consensus{
		identity:      did,
		network:       network,
		shardManager:  shardMgr,
		storage:       store,
		logger:        logger,
		state:         state,
		stateMgr:      stateMgr,
		ctx:           moduleCtx,
		cancel:        cancel,
		messageChan:   make(chan *packets.ConsensusPacket, 1000),
		processedMsgs: make(map[string]bool),
		validatorSet:  validatorSet,
		collector:     collector,
	}

	// Set initial state to `Uninitialized`
	stateMgr.SetState(ConsensusStateType, Uninitialized)
	return consensusModule
}

// Start begins the consensus operations.
func (cm *Consensus) Start() {
	cm.logger.Info("Starting Consensus Module")

	// Update state to `Starting`
	cm.stateMgr.SetState(ConsensusStateType, Starting)

	// Start leader election routine
	cm.wg.Add(1)
	go cm.leaderElectionRoutine()

	// Start network listener
	cm.wg.Add(1)
	go cm.listenToNetwork()

	// Start message processor
	cm.wg.Add(1)
	go cm.processMessages()

	// Update state to `Started` after successful startup
	cm.stateMgr.SetState(ConsensusStateType, Started)
}

// Shutdown gracefully shuts down the Consensus module.
func (cm *Consensus) Shutdown() error {
	cm.logger.Info("Shutting down Consensus Module")

	// Update state to `Stopping`
	cm.stateMgr.SetState(ConsensusStateType, Stopping)

	// Cancel the context and wait for all goroutines to finish
	cm.cancel()
	cm.wg.Wait()

	// Update state to `Stopped`
	cm.stateMgr.SetState(ConsensusStateType, Stopped)

	cm.logger.Info("Consensus Module shutdown complete")
	return nil
}

// leaderElectionRoutine periodically elects a new leader based on utility scores.
func (cm *Consensus) leaderElectionRoutine() {
	defer cm.wg.Done()
	ticker := time.NewTicker(time.Minute) // Elect leader every minute
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			// Elect a new leader using the ValidatorSet and MetricsCollector
			cm.validatorSet.ElectLeader(cm.collector)
			leader := cm.validatorSet.CurrentLeader()
			if leader == nil {
				cm.logger.Warn("No leader elected")
				continue
			}
			cm.setCurrentLeader(leader.ID)
			cm.logger.Info("New leader elected", zap.String("leaderID", leader.ID.String()))
			if leader.ID == cm.identity.PeerID {
				// If this node is the leader, start proposing blocks
				cm.wg.Add(1)
				go cm.proposeBlocks()
			}
		}
	}
}

// setCurrentLeader sets the current leader of the consensus module.
func (cm *Consensus) setCurrentLeader(leaderID peer.ID) {
	cm.currentLeader = leaderID
}

// listenToNetwork listens for messages from the network and handles them accordingly.
func (cm *Consensus) listenToNetwork() {
	defer cm.wg.Done()

	sub, err := cm.network.Topic.Subscribe()
	if err != nil {
		cm.logger.Error("Failed to subscribe to PubSub topic", zap.Error(err))
		return
	}
	defer sub.Cancel()

	for {
		select {
		case <-cm.ctx.Done():
			return
		default:
			msg, err := sub.Next(cm.ctx)
			if err != nil {
				if cm.ctx.Err() != nil {
					// Context canceled
					return
				}
				cm.logger.Error("Error reading from PubSub", zap.Error(err))
				continue
			}

			// Deserialize the consensus packet
			consensusPkt, err := packets.DeserializeConsensusPacket(msg.Data)
			if err != nil {
				cm.logger.Error("Failed to deserialize consensus packet", zap.Error(err))
				continue
			}

			// Verify signature
			valid, err := cm.verifySignature(consensusPkt)
			if err != nil || !valid {
				cm.logger.Warn("Invalid signature on consensus packet", zap.Error(err))
				continue
			}

			// Prevent duplicate processing
			msgID := fmt.Sprintf("%s:%x", consensusPkt.ValidatorID, consensusPkt.BlockHash)
			if cm.processedMsgs[msgID] {
				continue
			}
			cm.processedMsgs[msgID] = true

			// Send to processing channel
			select {
			case cm.messageChan <- consensusPkt:
			case <-cm.ctx.Done():
				return
			}
		}
	}
}

// processMessages processes consensus messages received from the network.
func (cm *Consensus) processMessages() {
	defer cm.wg.Done()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case pkt := <-cm.messageChan:
			switch pkt.Type {
			case packets.PacketTypeProposal:
				cm.handleProposal(pkt)
			case packets.PacketTypeApproval:
				cm.handleApproval(pkt)
			case packets.PacketTypeFinalization:
				cm.handleFinalization(pkt)
			default:
				cm.logger.Warn("Unknown packet type received", zap.Int("type", int(pkt.Type)))
			}
		}
	}
}

// handleProposal handles incoming proposal packets.
func (cm *Consensus) handleProposal(pkt *packets.ConsensusPacket) {
	// Only accept proposals from the current leader
	if pkt.ProposerID != cm.currentLeader {
		cm.logger.Warn("Received proposal from non-leader", zap.String("proposerID", pkt.ProposerID.String()))
		return
	}

	err := cm.state.AddProposal(pkt)
	if err != nil {
		cm.logger.Error("Failed to add proposal", zap.Error(err))
		return
	}

	// Approve the proposal if the node is a validator
	if cm.validatorSet.IsValidator(cm.identity.PeerID) {
		err = cm.ApproveProposal(pkt.BlockHash)
		if err != nil {
			cm.logger.Error("Failed to approve proposal", zap.Error(err))
		}
	}
}

// handleApproval handles incoming approval packets.
func (cm *Consensus) handleApproval(pkt *packets.ConsensusPacket) {
	err := cm.state.AddApproval(pkt)
	if err != nil {
		cm.logger.Error("Failed to add approval", zap.Error(err))
		return
	}

	// If this node is the leader, check for quorum and finalize the block
	if cm.identity.PeerID == cm.currentLeader {
		quorumSize := cm.validatorSet.QuorumSize()
		if cm.state.HasReachedQuorum(pkt.BlockHash, quorumSize) && !cm.state.IsFinalized(pkt.BlockHash) {
			// Finalize the block
			err = cm.FinalizeBlock(pkt.BlockHash)
			if err != nil {
				cm.logger.Error("Failed to finalize block", zap.Error(err))
			}
		}
	}
}

// handleFinalization handles incoming finalization packets.
func (cm *Consensus) handleFinalization(pkt *packets.ConsensusPacket) {
	err := cm.state.FinalizeBlock(pkt.BlockHash)
	if err != nil {
		cm.logger.Error("Failed to finalize block", zap.Error(err))
		return
	}
	cm.logger.Info("Block finalized", zap.String("blockHash", fmt.Sprintf("%x", pkt.BlockHash)))
}

// verifySignature verifies the signature of a consensus packet.
func (cm *Consensus) verifySignature(pkt *packets.ConsensusPacket) (bool, error) {
	var dataToVerify []byte
	switch pkt.Type {
	case packets.PacketTypeProposal:
		dataToVerify = append(pkt.BlockHash, pkt.BlockData...)
	case packets.PacketTypeApproval, packets.PacketTypeFinalization:
		dataToVerify = pkt.BlockHash
	default:
		return false, fmt.Errorf("unknown packet type: %d", pkt.Type)
	}

	var signerID peer.ID
	if pkt.Type == packets.PacketTypeProposal {
		signerID = pkt.ProposerID
	} else {
		signerID = pkt.ValidatorID
	}

	validator := cm.validatorSet.GetValidator(signerID)
	if validator == nil {
		return false, fmt.Errorf("validator not found: %s", signerID)
	}

	// Use the Verify function from the encryption package
	valid, err := encryption.Verify(dataToVerify, pkt.Signature, validator.PublicKey)
	if err != nil {
		return false, err
	}
	return valid, nil
}

// SignMessage signs a given message using the node's private key.
func (cm *Consensus) SignMessage(data []byte) (*encryption.BLSSignature, error) {
	signature, err := encryption.Sign(data, cm.identity.SigningPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}
	return signature, nil
}

// BroadcastMessage broadcasts a consensus packet to the network.
func (cm *Consensus) BroadcastMessage(pkt *packets.ConsensusPacket) error {
	if pkt == nil {
		return fmt.Errorf("consensus packet is nil")
	}
	serializedPkt, err := pkt.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize consensus packet: %w", err)
	}

	return cm.network.Topic.Publish(cm.ctx, serializedPkt)
}

// ProposeBlock proposes a new block to the network.
func (cm *Consensus) ProposeBlock(ctx context.Context, blockData []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	blockHash := consensus.HashData(blockData)
	cm.logger.Info("Proposing block", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))

	proposalPkt := &packets.ConsensusPacket{
		Type:       packets.PacketTypeProposal,
		ProposerID: cm.identity.PeerID,
		BlockHash:  blockHash,
		BlockData:  blockData,
	}

	dataToSign := append(blockHash, blockData...)
	signature, err := cm.SignMessage(dataToSign)
	if err != nil {
		return fmt.Errorf("failed to sign proposal packet: %w", err)
	}
	proposalPkt.Signature = signature

	err = cm.BroadcastMessage(proposalPkt)
	if err != nil {
		return fmt.Errorf("failed to broadcast proposal packet: %w", err)
	}

	err = cm.state.AddProposal(proposalPkt)
	if err != nil {
		return fmt.Errorf("failed to add proposal to state: %w", err)
	}

	return nil
}

// ApproveProposal approves a block proposal.
func (cm *Consensus) ApproveProposal(blockHash []byte) error {
	approvalPkt := &packets.ConsensusPacket{
		Type:        packets.PacketTypeApproval,
		ValidatorID: cm.identity.PeerID,
		BlockHash:   blockHash,
	}

	signature, err := cm.SignMessage(blockHash)
	if err != nil {
		return fmt.Errorf("failed to sign approval packet: %w", err)
	}
	approvalPkt.Signature = signature

	err = cm.BroadcastMessage(approvalPkt)
	if err != nil {
		return fmt.Errorf("failed to broadcast approval packet: %w", err)
	}

	err = cm.state.AddApproval(approvalPkt)
	if err != nil {
		return fmt.Errorf("failed to add approval to state: %w", err)
	}

	return nil
}

// FinalizeBlock finalizes a proposed block.
func (cm *Consensus) FinalizeBlock(blockHash []byte) error {
	finalizationPkt := &packets.ConsensusPacket{
		Type:        packets.PacketTypeFinalization,
		ValidatorID: cm.identity.PeerID,
		BlockHash:   blockHash,
	}

	signature, err := cm.SignMessage(blockHash)
	if err != nil {
		return fmt.Errorf("failed to sign finalization packet: %w", err)
	}
	finalizationPkt.Signature = signature

	err = cm.BroadcastMessage(finalizationPkt)
	if err != nil {
		return fmt.Errorf("failed to broadcast finalization packet: %w", err)
	}

	err = cm.state.FinalizeBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to finalize block: %w", err)
	}

	cm.logger.Info("Block finalized", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))
	return nil
}

func (cm *Consensus) proposeBlocks() {
	defer cm.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			blockData := []byte(fmt.Sprintf("Block proposed at %s by %s", time.Now().String(), cm.identity.PeerID.String()))
			err := cm.ProposeBlock(cm.ctx, blockData)
			if err != nil {
				cm.logger.Error("Failed to propose block", zap.Error(err))
			}
		}
	}
}
