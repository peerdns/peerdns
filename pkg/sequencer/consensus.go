package sequencer

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/metrics"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/packets"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"
	"go.uber.org/zap"
	"sync"
	"time"
)

// Consensus represents the sequencer's consensus module.
type Consensus struct {
	identity      *accounts.Account
	logger        logger.Logger
	validatorSet  *consensus.ValidatorSet
	network       *networking.Network
	shardManager  *sharding.ShardManager
	storage       *storage.Db
	state         *consensus.State
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	messageChan   chan *packets.ConsensusPacket
	processedMsgs map[string]bool
	currentLeader peer.ID
	collector     *metrics.Collector
}

// NewConsensus creates a new Consensus module for the sequencer.
func NewConsensus(ctx context.Context, identity *accounts.Account, network *networking.Network, shardMgr *sharding.ShardManager, store *storage.Db, logger logger.Logger, validatorSet *consensus.ValidatorSet, collector *metrics.Collector) *Consensus {
	state := consensus.NewState(store, logger)
	moduleCtx, cancel := context.WithCancel(ctx)

	// Initialize the Consensus module
	consensusModule := &Consensus{
		identity:      identity,
		network:       network,
		shardManager:  shardMgr,
		storage:       store,
		logger:        logger,
		state:         state,
		ctx:           moduleCtx,
		cancel:        cancel,
		messageChan:   make(chan *packets.ConsensusPacket, 1000),
		processedMsgs: make(map[string]bool),
		validatorSet:  validatorSet,
		collector:     collector,
		currentLeader: identity.PeerID, // Set sequencer as the leader initially
	}

	return consensusModule
}

// Start begins the consensus operations.
func (c *Consensus) Start() {
	c.logger.Info("Starting Consensus Module")

	// Since this sequencer is the leader, start proposing blocks directly
	c.wg.Add(1)
	go c.proposeBlocks()

	// Start network listener
	c.wg.Add(1)
	go c.listenToNetwork()

	// Start message processor
	c.wg.Add(1)
	go c.processMessages()
}

// Shutdown gracefully shuts down the Consensus module.
func (c *Consensus) Shutdown() {
	c.logger.Info("Shutting down Consensus Module")

	// Cancel the context and wait for all goroutines to finish
	c.cancel()
	c.wg.Wait()

	c.logger.Info("Consensus Module shutdown complete")
}

// listenToNetwork listens for messages from the network and handles them accordingly.
func (c *Consensus) listenToNetwork() {
	defer c.wg.Done()

	sub, err := c.network.Topic.Subscribe()
	if err != nil {
		c.logger.Error("Failed to subscribe to PubSub topic", zap.Error(err))
		return
	}
	defer sub.Cancel()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			msg, err := sub.Next(c.ctx)
			if err != nil {
				if c.ctx.Err() != nil {
					return
				}
				c.logger.Error("Error reading from PubSub", zap.Error(err))
				continue
			}

			consensusPkt, err := packets.DeserializeConsensusPacket(msg.Data)
			if err != nil {
				c.logger.Error("Failed to deserialize consensus packet", zap.Error(err))
				continue
			}

			valid, err := c.verifySignature(consensusPkt)
			if err != nil || !valid {
				c.logger.Warn("Invalid signature on consensus packet", zap.Error(err))
				continue
			}

			msgID := fmt.Sprintf("%s:%x", consensusPkt.ValidatorID, consensusPkt.BlockHash)
			if c.processedMsgs[msgID] {
				continue
			}
			c.processedMsgs[msgID] = true

			select {
			case c.messageChan <- consensusPkt:
			case <-c.ctx.Done():
				return
			}
		}
	}
}

// processMessages processes consensus messages received from the network.
func (c *Consensus) processMessages() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case pkt := <-c.messageChan:
			switch pkt.Type {
			case packets.PacketTypeProposal:
				c.handleProposal(pkt)
			case packets.PacketTypeApproval:
				c.handleApproval(pkt)
			case packets.PacketTypeFinalization:
				c.handleFinalization(pkt)
			default:
				c.logger.Warn("Unknown packet type received", zap.Int("type", int(pkt.Type)))
			}
		}
	}
}

// handleProposal handles incoming proposal packets.
func (c *Consensus) handleProposal(pkt *packets.ConsensusPacket) {
	if pkt.ProposerID != c.currentLeader {
		c.logger.Warn("Received proposal from non-leader", zap.String("proposerID", pkt.ProposerID.String()))
		return
	}

	err := c.state.AddProposal(pkt)
	if err != nil {
		c.logger.Error("Failed to add proposal", zap.Error(err))
		return
	}

	if c.validatorSet.IsValidator(c.identity.PeerID) {
		err = c.approveBlock(types.Hash(pkt.BlockHash))
		if err != nil {
			c.logger.Error("Failed to approve proposal", zap.Error(err))
		}
	}
}

// handleApproval handles incoming approval packets.
func (c *Consensus) handleApproval(pkt *packets.ConsensusPacket) {
	err := c.state.AddApproval(pkt)
	if err != nil {
		c.logger.Error("Failed to add approval", zap.Error(err))
		return
	}

	if c.identity.PeerID == c.currentLeader {
		quorumSize := c.validatorSet.QuorumSize()
		if c.state.HasReachedQuorum(pkt.BlockHash, quorumSize) && !c.state.IsFinalized(pkt.BlockHash) {
			err = c.finalizeBlock(types.Hash(pkt.BlockHash))
			if err != nil {
				c.logger.Error("Failed to finalize block", zap.Error(err))
			}
		}
	}
}

// handleFinalization handles incoming finalization packets.
func (c *Consensus) handleFinalization(pkt *packets.ConsensusPacket) {
	err := c.state.FinalizeBlock(pkt.BlockHash)
	if err != nil {
		c.logger.Error("Failed to finalize block", zap.Error(err))
		return
	}
	c.logger.Info("Block finalized", zap.String("blockHash", fmt.Sprintf("%x", pkt.BlockHash)))
}

// verifySignature verifies the signature of a consensus packet.
func (c *Consensus) verifySignature(pkt *packets.ConsensusPacket) (bool, error) {
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

	validator := c.validatorSet.GetValidator(signerID)
	if validator == nil {
		return false, fmt.Errorf("validator not found: %s", signerID)
	}

	valid, err := validator.Signer(pkt.Signer).Verify(dataToVerify, pkt.Signature)
	if err != nil {
		return false, err
	}
	return valid, nil
}

func (c *Consensus) proposeBlocks() {
	defer c.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			block := &types.Block{Hash: types.Hash(types.HashData([]byte(fmt.Sprintf("Block proposed at %s by %s", time.Now().String(), c.identity.PeerID.String()))))}
			blockData, err := block.Serialize()
			if err != nil {
				c.logger.Error("Failed to serialize block", zap.Error(err))
				continue
			}
			err = c.proposeBlock(c.ctx, block, blockData)
			if err != nil {
				c.logger.Error("Failed to propose block", zap.Error(err))
			}
		}
	}
}

// approveBlock sends an approval for the proposed block.
func (c *Consensus) approveBlock(blockHash types.Hash) error {
	approvalPkt := &packets.ConsensusPacket{
		Type:        packets.PacketTypeApproval,
		Signer:      signatures.BlsSignerType,
		ValidatorID: c.identity.PeerID,
		BlockHash:   blockHash[:],
	}

	signature, err := c.identity.Sign(signatures.BlsSignerType, blockHash[:])
	if err != nil {
		return fmt.Errorf("failed to sign approval packet: %w", err)
	}
	approvalPkt.Signature = signature

	serializedPkt, err := approvalPkt.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize approval packet: %w", err)
	}
	err = c.network.Topic.Publish(c.ctx, serializedPkt)
	if err != nil {
		return fmt.Errorf("failed to broadcast approval packet: %w", err)
	}

	err = c.state.AddApproval(approvalPkt)
	if err != nil {
		return fmt.Errorf("failed to add approval to state: %w", err)
	}

	return nil
}

// finalizeBlock finalizes a proposed block.
func (c *Consensus) finalizeBlock(blockHash types.Hash) error {
	finalizationPkt := &packets.ConsensusPacket{
		Type:        packets.PacketTypeFinalization,
		Signer:      signatures.BlsSignerType,
		ValidatorID: c.identity.PeerID,
		BlockHash:   blockHash[:],
	}

	signature, err := c.identity.Sign(signatures.BlsSignerType, blockHash[:])
	if err != nil {
		return fmt.Errorf("failed to sign finalization packet: %w", err)
	}
	finalizationPkt.Signature = signature

	serializedPkt, err := finalizationPkt.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize finalization packet: %w", err)
	}
	err = c.network.Topic.Publish(c.ctx, serializedPkt)
	if err != nil {
		return fmt.Errorf("failed to broadcast finalization packet: %w", err)
	}

	err = c.state.FinalizeBlock(blockHash[:])
	if err != nil {
		return fmt.Errorf("failed to finalize block: %w", err)
	}

	c.logger.Info("Block finalized", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))
	return nil
}

// proposeBlock proposes a new block to the network.
func (c *Consensus) proposeBlock(ctx context.Context, block *types.Block, blockData []byte) error {
	blockHash := block.Hash
	c.logger.Info("Proposing block", zap.String("blockHash", fmt.Sprintf("%x", blockHash)))

	proposalPkt := &packets.ConsensusPacket{
		Type:       packets.PacketTypeProposal,
		Signer:     signatures.BlsSignerType,
		ProposerID: c.identity.PeerID,
		BlockHash:  blockHash[:],
		BlockData:  blockData,
	}

	signature, err := c.identity.Sign(signatures.BlsSignerType, append(blockHash[:], blockData...))
	if err != nil {
		return fmt.Errorf("failed to sign proposal packet: %w", err)
	}
	proposalPkt.Signature = signature

	serializedPkt, err := proposalPkt.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize proposal packet: %w", err)
	}
	err = c.network.Topic.Publish(ctx, serializedPkt)
	if err != nil {
		return fmt.Errorf("failed to broadcast proposal packet: %w", err)
	}

	err = c.state.AddProposal(proposalPkt)
	if err != nil {
		return fmt.Errorf("failed to add proposal to state: %w", err)
	}

	return nil
}
