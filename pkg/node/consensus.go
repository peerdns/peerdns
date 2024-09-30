// pkg/node/consensus_module.go
package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/messages"
	"github.com/peerdns/peerdns/pkg/metrics"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/privacy"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/validator"
	"go.uber.org/zap"
)

type Consensus struct {
	identity         *identity.DID
	network          *networking.P2PNetwork
	shardManager     *sharding.ShardManager
	privacyMgr       *privacy.PrivacyManager
	storage          *storage.Db
	logger           logger.Logger
	state            *consensus.ConsensusState
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	messageChan      chan *messages.ConsensusMessage
	processedMsgs    map[string]bool // To prevent processing duplicate messages
	validatorSet     *consensus.ValidatorSet
	currentLeader    peer.ID
	metricsCollector *metrics.MetricsCollector
}

func NewConsensus(ctx context.Context, did *identity.DID, network *networking.P2PNetwork, shardMgr *sharding.ShardManager, privacyMgr *privacy.PrivacyManager, store *storage.Db, logger logger.Logger, validatorInstance *validator.Validator, metricsCollector *metrics.MetricsCollector) *Consensus {
	state := consensus.NewConsensusState(store, logger)
	moduleCtx, cancel := context.WithCancel(ctx)
	return &Consensus{
		identity:         did,
		network:          network,
		shardManager:     shardMgr,
		privacyMgr:       privacyMgr,
		storage:          store,
		logger:           logger,
		state:            state,
		ctx:              moduleCtx,
		cancel:           cancel,
		messageChan:      make(chan *messages.ConsensusMessage, 1000),
		processedMsgs:    make(map[string]bool),
		validatorSet:     validatorInstance.ValidatorSet,
		metricsCollector: metricsCollector,
	}
}

func (cm *Consensus) Start() {
	cm.logger.Info("Starting Consensus Module")

	// Start leader election routine
	cm.wg.Add(1)
	go cm.leaderElectionRoutine()

	// Start network listener
	cm.wg.Add(1)
	go cm.listenToNetwork()

	// Start message processor
	cm.wg.Add(1)
	go cm.processMessages()
}

func (cm *Consensus) Shutdown() error {
	cm.cancel()
	cm.wg.Wait()
	cm.logger.Info("Consensus Module shutdown complete")
	return nil
}

// leaderElectionRoutine periodically elects a new leader based on utility scores
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
			cm.validatorSet.ElectLeader(cm.metricsCollector)
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

func (cm *Consensus) setCurrentLeader(leaderID peer.ID) {
	cm.currentLeader = leaderID
}

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

			// Deserialize the consensus message
			consensusMsg, err := messages.DeserializeConsensusMessage(msg.Data)
			if err != nil {
				cm.logger.Error("Failed to deserialize consensus message", zap.Error(err))
				continue
			}

			// Verify signature
			valid, err := cm.verifySignature(consensusMsg)
			if err != nil || !valid {
				cm.logger.Warn("Invalid signature on consensus message", zap.Error(err))
				continue
			}

			// Prevent duplicate processing
			msgID := fmt.Sprintf("%s:%x", consensusMsg.ValidatorID, consensusMsg.BlockHash)
			if cm.processedMsgs[msgID] {
				continue
			}
			cm.processedMsgs[msgID] = true

			// Send to processing channel
			select {
			case cm.messageChan <- consensusMsg:
			case <-cm.ctx.Done():
				return
			}
		}
	}
}

func (cm *Consensus) processMessages() {
	defer cm.wg.Done()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case msg := <-cm.messageChan:
			switch msg.Type {
			case messages.ProposalMessage:
				cm.handleProposal(msg)
			case messages.ApprovalMessage:
				cm.handleApproval(msg)
			case messages.FinalizationMessage:
				cm.handleFinalization(msg)
			default:
				cm.logger.Warn("Unknown message type received", zap.Int("type", int(msg.Type)))
			}
		}
	}
}

func (cm *Consensus) handleProposal(msg *messages.ConsensusMessage) {
	// Only accept proposals from the current leader
	if msg.ProposerID != cm.currentLeader {
		cm.logger.Warn("Received proposal from non-leader", zap.String("proposerID", msg.ProposerID.String()))
		return
	}

	err := cm.state.AddProposal(msg)
	if err != nil {
		cm.logger.Error("Failed to add proposal", zap.Error(err))
		return
	}

	// Approve the proposal if the node is a validator
	if cm.validatorSet.IsValidator(cm.identity.PeerID) {
		err = cm.ApproveProposal(msg.BlockHash)
		if err != nil {
			cm.logger.Error("Failed to approve proposal", zap.Error(err))
		}
	}
}

func (cm *Consensus) handleApproval(msg *messages.ConsensusMessage) {
	err := cm.state.AddApproval(msg)
	if err != nil {
		cm.logger.Error("Failed to add approval", zap.Error(err))
		return
	}

	// If this node is the leader, check for quorum and finalize the block
	if cm.identity.PeerID == cm.currentLeader {
		quorumSize := cm.validatorSet.QuorumSize()
		if cm.state.HasReachedQuorum(msg.BlockHash, quorumSize) && !cm.state.IsFinalized(msg.BlockHash) {
			// Finalize the block
			err = cm.FinalizeBlock(msg.BlockHash)
			if err != nil {
				cm.logger.Error("Failed to finalize block", zap.Error(err))
			}
		}
	}
}

func (cm *Consensus) handleFinalization(msg *messages.ConsensusMessage) {
	err := cm.state.FinalizeBlock(msg.BlockHash)
	if err != nil {
		cm.logger.Error("Failed to finalize block", zap.Error(err))
		return
	}
	cm.logger.Info("Block finalized", zap.String("blockHash", fmt.Sprintf("%x", msg.BlockHash)))
}

func (cm *Consensus) verifySignature(msg *messages.ConsensusMessage) (bool, error) {
	var dataToVerify []byte
	switch msg.Type {
	case messages.ProposalMessage:
		dataToVerify = append(msg.BlockHash, msg.BlockData...)
	case messages.ApprovalMessage, messages.FinalizationMessage:
		dataToVerify = msg.BlockHash
	default:
		return false, fmt.Errorf("unknown message type: %d", msg.Type)
	}

	var signerID peer.ID
	if msg.Type == messages.ProposalMessage {
		signerID = msg.ProposerID
	} else {
		signerID = msg.ValidatorID
	}

	vdator := cm.validatorSet.GetValidator(signerID)
	if vdator == nil {
		return false, fmt.Errorf("validator not found: %s", signerID)
	}

	// Use the Verify function from the encryption package
	valid, err := encryption.Verify(dataToVerify, msg.Signature, vdator.PublicKey)
	if err != nil {
		return false, err
	}
	return valid, nil
}

func (cm *Consensus) SignMessage(data []byte) (*encryption.BLSSignature, error) {
	// Use the Sign function from the encryption package
	signature, err := encryption.Sign(data, cm.identity.SigningPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}
	return signature, nil
}

func (cm *Consensus) BroadcastMessage(ctx context.Context, msg *messages.ConsensusMessage) error {
	serializedMsg, err := msg.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize consensus message: %w", err)
	}

	// Publish to PubSub topic
	return cm.network.Topic.Publish(ctx, serializedMsg)
}

func (cm *Consensus) ProposeBlock(ctx context.Context, blockData []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	blockHash := consensus.HashData(blockData)
	cm.logger.Info("Proposing block", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))

	// Create a proposal message
	proposalMsg := &messages.ConsensusMessage{
		Type:       messages.ProposalMessage,
		ProposerID: cm.identity.PeerID,
		BlockHash:  blockHash,
		BlockData:  blockData,
	}

	// Sign the data (block hash + block data)
	dataToSign := append(blockHash, blockData...)
	signature, err := cm.SignMessage(dataToSign)
	if err != nil {
		return fmt.Errorf("failed to sign proposal message: %w", err)
	}
	proposalMsg.Signature = signature

	// Broadcast the proposal
	err = cm.BroadcastMessage(ctx, proposalMsg)
	if err != nil {
		return fmt.Errorf("failed to broadcast proposal message: %w", err)
	}

	// Add proposal to state
	err = cm.state.AddProposal(proposalMsg)
	if err != nil {
		return fmt.Errorf("failed to add proposal to state: %w", err)
	}

	return nil
}

func (cm *Consensus) ApproveProposal(blockHash []byte) error {
	// Create approval message
	approvalMsg := &messages.ConsensusMessage{
		Type:        messages.ApprovalMessage,
		ValidatorID: cm.identity.PeerID,
		BlockHash:   blockHash,
	}

	// Sign the block hash
	signature, err := cm.SignMessage(blockHash)
	if err != nil {
		return fmt.Errorf("failed to sign approval message: %w", err)
	}
	approvalMsg.Signature = signature

	// Broadcast the approval
	err = cm.BroadcastMessage(cm.ctx, approvalMsg)
	if err != nil {
		return fmt.Errorf("failed to broadcast approval message: %w", err)
	}

	// Add approval to state
	err = cm.state.AddApproval(approvalMsg)
	if err != nil {
		return fmt.Errorf("failed to add approval to state: %w", err)
	}

	return nil
}

func (cm *Consensus) FinalizeBlock(blockHash []byte) error {
	// Create finalization message
	finalizationMsg := &messages.ConsensusMessage{
		Type:        messages.FinalizationMessage,
		ValidatorID: cm.identity.PeerID,
		BlockHash:   blockHash,
	}

	// Sign the block hash
	signature, err := cm.SignMessage(blockHash)
	if err != nil {
		return fmt.Errorf("failed to sign finalization message: %w", err)
	}
	finalizationMsg.Signature = signature

	// Broadcast the finalization message
	err = cm.BroadcastMessage(cm.ctx, finalizationMsg)
	if err != nil {
		return fmt.Errorf("failed to broadcast finalization message: %w", err)
	}

	// Finalize the block in the state
	err = cm.state.FinalizeBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to finalize block: %w", err)
	}

	cm.logger.Info("Block finalized", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))
	return nil
}

func (cm *Consensus) proposeBlocks() {
	defer cm.wg.Done()
	ticker := time.NewTicker(30 * time.Second) // Propose a block every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			// Create dummy block data for demonstration
			blockData := []byte(fmt.Sprintf("Block proposed at %s by %s", time.Now().String(), cm.identity.PeerID.String()))
			err := cm.ProposeBlock(cm.ctx, blockData)
			if err != nil {
				cm.logger.Error("Failed to propose block", zap.Error(err))
			}
		}
	}
}
