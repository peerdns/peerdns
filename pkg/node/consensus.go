// pkg/node/consensus_module.go
package node

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/messages"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/privacy"
	"github.com/peerdns/peerdns/pkg/sharding"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/validator"
	"go.uber.org/zap"
)

type ConsensusModule struct {
	identity      *identity.DID
	network       *networking.P2PNetwork
	shardManager  *sharding.ShardManager
	privacyMgr    *privacy.PrivacyManager
	storage       *storage.Db
	logger        *zap.Logger
	state         *consensus.ConsensusState
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	messageChan   chan *messages.ConsensusMessage
	processedMsgs map[string]bool // To prevent processing duplicate messages
	validatorSet  *consensus.ValidatorSet
	validatorInfo *consensus.Validator
}

func NewConsensusModule(ctx context.Context, did *identity.DID, network *networking.P2PNetwork, shardMgr *sharding.ShardManager, privacyMgr *privacy.PrivacyManager, store *storage.Db, logger *zap.Logger, validatorInstance *validator.Validator) *ConsensusModule {
	state := consensus.NewConsensusState(store, logger)
	moduleCtx, cancel := context.WithCancel(ctx)
	return &ConsensusModule{
		identity:      did,
		network:       network,
		shardManager:  shardMgr,
		privacyMgr:    privacyMgr,
		storage:       store,
		logger:        logger,
		state:         state,
		ctx:           moduleCtx,
		cancel:        cancel,
		messageChan:   make(chan *messages.ConsensusMessage, 1000),
		processedMsgs: make(map[string]bool),
		validatorSet:  validatorInstance.ValidatorSet,
		validatorInfo: validatorInstance.ValidatorInfo,
	}
}

func (cm *ConsensusModule) Start() {
	cm.logger.Info("Starting Consensus Module")

	cm.wg.Add(1)
	go cm.listenToNetwork()

	cm.wg.Add(1)
	go cm.processMessages()
}

func (cm *ConsensusModule) Shutdown() {
	cm.cancel()
	cm.wg.Wait()
	cm.logger.Info("Consensus Module shutdown complete")
}

func (cm *ConsensusModule) listenToNetwork() {
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
				cm.logger.Error("Error reading from PubSub", zap.Error(err))
				continue
			}

			// Decrypt the message
			messageData, err := cm.privacyMgr.Decrypt(msg.Data)
			if err != nil {
				cm.logger.Error("Failed to decrypt message", zap.Error(err))
				continue
			}

			var consensusMsg messages.ConsensusMessage
			err = gob.NewDecoder(bytes.NewReader(messageData)).Decode(&consensusMsg)
			if err != nil {
				cm.logger.Error("Failed to decode consensus message", zap.Error(err))
				continue
			}

			// Verify signature
			valid, err := cm.verifySignature(&consensusMsg)
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
			cm.messageChan <- &consensusMsg
		}
	}
}

func (cm *ConsensusModule) processMessages() {
	defer cm.wg.Done()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case msg := <-cm.messageChan:
			// Process consensus message
			err := cm.state.AddProposal(msg)
			if err != nil {
				cm.logger.Error("Failed to add proposal", zap.Error(err))
				continue
			}

			// Check for quorum
			if cm.state.HasReachedQuorum(msg.BlockHash, cm.state.QuorumThreshold()) {
				// Finalize the block
				err = cm.state.FinalizeBlock(msg.BlockHash)
				if err != nil {
					cm.logger.Error("Failed to finalize block", zap.Error(err))
					continue
				}

				cm.logger.Info("Block finalized", zap.ByteString("blockHash", msg.BlockHash))
			}
		}
	}
}

func (cm *ConsensusModule) verifySignature(msg *messages.ConsensusMessage) (bool, error) {
	validator := cm.validatorSet.GetValidator(msg.ValidatorID)
	if validator == nil {
		return false, fmt.Errorf("validator not found: %s", msg.ValidatorID)
	}

	isValid := encryption.Verify(msg.BlockHash, msg.Signature)
	return isValid, nil
}

func (cm *ConsensusModule) SignMessage(blockHash []byte) (*messages.ConsensusMessage, error) {
	signature := encryption.Sign(blockHash)
	msg := &messages.ConsensusMessage{
		BlockHash:   blockHash,
		ValidatorID: peer.ID(cm.identity.ID),
		Signature:   signature,
	}
	return msg, nil
}

func (cm *ConsensusModule) BroadcastMessage(msg *messages.ConsensusMessage) error {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode consensus message: %w", err)
	}

	// Encrypt the message
	encryptedMessage, err := cm.privacyMgr.Encrypt(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to encrypt message: %w", err)
	}

	// Publish to PubSub topic
	return cm.network.Topic.Publish(cm.ctx, encryptedMessage)
}
