// pkg/consensus/consensus_module.go
package consensus

/*
import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/messages"
	"log"
	"sync"

	"shponu/pkg/identity"
	"shponu/pkg/networking"
	"shponu/pkg/privacy"
	"shponu/pkg/sharding"
	"shponu/pkg/storage"
	"shponu/pkg/utility"
)

// SignedMessage represents a message signed by a validator.
type SignedMessage struct {
	Content   []byte
	Signature []byte
}

// ConsensusModule manages the consensus process for a validator.
type ConsensusModule struct {
	identity      *identity.DID
	network       *networking.P2PNetwork
	utilityCalc   *utility.UtilityCalculator
	shardManager  *sharding.ShardManager
	privacyMgr    *privacy.PrivacyManager
	storage       *storage.Db
	logger        *log.Logger
	state         *ConsensusState
	subscription  *networking.PubSubService
	wg            sync.WaitGroup
	stopChan      chan struct{}
	messageChan   chan SignedMessage
	mutex         sync.Mutex
	processedMsgs map[string]bool // To prevent processing duplicate messages
}

// NewConsensusModule initializes a new ConsensusModule.
func NewConsensusModule(did *identity.DID, network *networking.P2PNetwork, utilityCalc *utility.UtilityCalculator, shardMgr *sharding.ShardManager, privacyMgr *privacy.PrivacyManager, store *storage.Db, logger *log.Logger) *ConsensusModule {
	state := NewConsensusState(store, logger)
	return &ConsensusModule{
		identity:      did,
		network:       network,
		utilityCalc:   utilityCalc,
		shardManager:  shardMgr,
		privacyMgr:    privacyMgr,
		storage:       store,
		logger:        logger,
		state:         state,
		stopChan:      make(chan struct{}),
		messageChan:   make(chan SignedMessage, 1000),
		processedMsgs: make(map[string]bool),
	}
}

// Start begins the consensus process.
func (cm *ConsensusModule) Start() {
	cm.wg.Add(1)
	go cm.listenToPubSub()

	cm.wg.Add(1)
	go cm.processMessages()
}

// Stop gracefully stops the consensus process.
func (cm *ConsensusModule) Stop() {
	close(cm.stopChan)
	cm.wg.Wait()
}

// listenToPubSub subscribes to the PubSub topic and listens for incoming messages.
func (cm *ConsensusModule) listenToPubSub() {
	defer cm.wg.Done()

	sub, err := cm.network.PubSub.Subscribe("shponu-topic")
	if err != nil {
		cm.logger.Printf("Failed to subscribe to PubSub: %v", err)
		return
	}
	defer sub.Cancel()

	for {
		select {
		case <-cm.stopChan:
			return
		default:
			msg, err := sub.Next(cm.network.Ctx)
			if err != nil {
				cm.logger.Printf("Error reading from PubSub: %v", err)
				continue
			}

			var netMsg networking.NetworkMessage
			err = networking.DeserializeMessage(msg.Data, &netMsg)
			if err != nil {
				cm.logger.Printf("Failed to deserialize network message: %v", err)
				continue
			}

			if netMsg.Type != networking.MessageTypeProposal {
				continue // Only handle proposal messages
			}

			// Verify the signature
			isValid, err := cm.verifySignature(netMsg.Content, netMsg.From, netMsg.Signature)
			if err != nil {
				cm.logger.Printf("Signature verification error: %v", err)
				continue
			}
			if !isValid {
				cm.logger.Printf("Invalid signature from %s", netMsg.From)
				continue
			}

			// Prevent duplicate processing
			messageHash := fmt.Sprintf("%s:%x", netMsg.From, netMsg.Content)
			cm.mutex.Lock()
			if _, exists := cm.processedMsgs[messageHash]; exists {
				cm.mutex.Unlock()
				continue
			}
			cm.processedMsgs[messageHash] = true
			cm.mutex.Unlock()

			// Send the message to the processing channel
			cm.messageChan <- SignedMessage{
				Content:   netMsg.Content,
				Signature: netMsg.Signature,
			}
		}
	}
}

// processMessages handles incoming signed messages.
func (cm *ConsensusModule) processMessages() {
	defer cm.wg.Done()

	for {
		select {
		case <-cm.stopChan:
			return
		case msg := <-cm.messageChan:
			// Deserialize the message
			var proposal Proposal
			err := gob.NewDecoder(bytes.NewReader(msg.Content)).Decode(&proposal)
			if err != nil {
				cm.logger.Printf("Failed to decode proposal: %v", err)
				continue
			}

			// Add proposal to the state
			err = cm.state.AddProposal(&messages.ConsensusMessage{
				BlockHash:   proposal.BlockHash,
				ValidatorID: peerIDFromDID(cm.identity),
				Signature:   &encryption.BLSSignature{Signature: msg.Signature},
			})
			if err != nil {
				cm.logger.Printf("Failed to add proposal: %v", err)
				continue
			}

			// Check for quorum
			if cm.state.HasReachedQuorum(proposal.BlockHash, 7) { // Example quorum size
				// Finalize the block
				err = cm.state.FinalizeBlock(proposal.BlockHash)
				if err != nil {
					cm.logger.Printf("Failed to finalize block: %v", err)
					continue
				}

				// Broadcast finalization message
				finalMsg := FinalizationMessage{
					BlockHash: proposal.BlockHash,
					Signature: cm.signData(proposal.BlockHash),
				}
				finalMsgBytes, err := serializeFinalizationMessage(finalMsg)
				if err != nil {
					cm.logger.Printf("Failed to serialize finalization message: %v", err)
					continue
				}

				err = cm.network.BroadcastMessage(finalMsgBytes)
				if err != nil {
					cm.logger.Printf("Failed to broadcast finalization message: %v", err)
					continue
				}

				cm.logger.Printf("Block %x finalized and broadcasted.", proposal.BlockHash)
			}
		}
	}
}

// SignMessage signs the given message content using the validator's private key.
func (cm *ConsensusModule) SignMessage(content []byte) (*SignedMessage, error) {
	signature := cm.identity.PrivateKey.SignHash(bls.HashFromBytes(content))
	return &SignedMessage{
		Content:   content,
		Signature: signature.Serialize(),
	}, nil
}

// BroadcastMessage serializes and broadcasts a signed message to the network.
func (cm *ConsensusModule) BroadcastMessage(msg *SignedMessage) error {
	netMsg := networking.NetworkMessage{
		Type:      networking.MessageTypeProposal,
		From:      cm.identity.ID,
		Content:   msg.Content,
		Signature: msg.Signature,
	}
	serializedMsg, err := networking.SerializeMessage(netMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize network message: %w", err)
	}

	return cm.network.BroadcastMessage(serializedMsg)
}

// verifySignature verifies the signature of a message.
func (cm *ConsensusModule) verifySignature(content []byte, from string, signature []byte) (bool, error) {
	// Retrieve the sender's DID
	identityMgr := identity.NewIdentityManager(cm.storage)
	senderDID, err := identityMgr.GetDID(from)
	if err != nil {
		return false, fmt.Errorf("failed to get sender DID: %w", err)
	}

	// Deserialize the signature
	var sig bls.Sign
	err = sig.Deserialize(signature)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize signature: %w", err)
	}

	// Verify the signature
	isValid := sig.VerifyHash(senderDID.PublicKey, bls.HashFromBytes(content))
	return isValid, nil
}

// signData signs arbitrary data using the validator's private key.
func (cm *ConsensusModule) signData(data []byte) []byte {
	signature := cm.identity.PrivateKey.SignHash(bls.HashFromBytes(data))
	return signature.Serialize()
}

// peerIDFromDID converts a DID to a peer.ID (placeholder implementation).
func peerIDFromDID(did *identity.DID) peer.ID {
	// Placeholder: In a real implementation, map DID to peer.ID
	// For now, return a dummy peer.ID
	return "dummy-peer-id"
}

// serializeFinalizationMessage serializes a FinalizationMessage.
func serializeFinalizationMessage(msg FinalizationMessage) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode finalization message: %w", err)
	}
	return buf.Bytes(), nil
}

// Proposal represents a block proposal.
type Proposal struct {
	BlockHash []byte
	// Additional fields can be added as needed
}

// FinalizationMessage represents a block finalization.
type FinalizationMessage struct {
	BlockHash []byte
	Signature []byte
}
*/
