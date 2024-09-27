// pkg/consensus/consensus_test.go
package consensus

import (
	"context"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/messages"
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/stretchr/testify/assert"
)

// MockBlockFinalizer is a mock implementation for block finalization.
type MockBlockFinalizer struct {
	finalizedBlocks map[string][]byte
	mu              sync.Mutex
}

// NewMockBlockFinalizer initializes a new MockBlockFinalizer.
func NewMockBlockFinalizer() *MockBlockFinalizer {
	return &MockBlockFinalizer{
		finalizedBlocks: make(map[string][]byte),
	}
}

// Finalize finalizes a block.
func (bf *MockBlockFinalizer) Finalize(blockHash []byte, blockData []byte) error {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.finalizedBlocks[string(blockHash)] = blockData
	return nil
}

// IsFinalized checks if a block is finalized.
func (bf *MockBlockFinalizer) IsFinalized(blockHash []byte) bool {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	_, exists := bf.finalizedBlocks[string(blockHash)]
	return exists
}

// waitForBroadcastedMessages waits until the mock P2P network has broadcasted at least 'expected' messages.
// It fails the test if the messages are not broadcasted within the 'timeout' duration.
func waitForBroadcastedMessages(t *testing.T, mockP2P *networking.MockP2PNetwork, expected int, timeout time.Duration) {
	t.Helper()
	count := 0
	for {
		select {
		case <-mockP2P.Ch():
			count++
			t.Logf("Received broadcasted message #%d", count)
			if count >= expected {
				return
			}
		case <-time.After(timeout):
			messages := mockP2P.GetBroadcastedMessages()
			t.Fatalf("Timeout waiting for %d broadcasted messages, only %d received", expected, len(messages))
		}
	}
}

// printValidators logs all validators in the ValidatorSet.
func printValidators(t *testing.T, validators *ValidatorSet) {
	t.Helper()
	t.Logf("Current Validators:")
	for _, validator := range validators.GetAllValidators() {
		t.Logf("- ID: %s, PublicKey: %v", validator.ID, validator.PublicKey)
	}
}

// setupTestConsensus initializes the ConsensusProtocol with a mock P2P network.
func setupTestConsensus(t *testing.T) (*ConsensusProtocol, context.Context, *ValidatorSet, *storage.Manager, peer.ID) {
	logger := log.New(os.Stdout, "ConsensusTest: ", log.LstdFlags)
	ctx := context.Background()

	// Initialize BLS library for testing
	err := encryption.InitBLS()
	require.NoError(t, err)

	// Create validators and elect a leader
	validators := NewValidatorSet(logger)

	// Generate BLS keys for three validators
	privKey1, pk1, err := encryption.GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys for validator 1")

	privKey2, pk2, err := encryption.GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys for validator 2")

	privKey3, pk3, err := encryption.GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys for mock-host")

	// Define valid peer IDs
	validator1ID, err := peer.Decode("QmWmyoCVmCuB8h5eX7ZpZ1eZC8F3JY4A9uYjnrF1i5jdbY")
	assert.NoError(t, err, "Failed to decode validator1ID")

	validator2ID, err := peer.Decode("QmU5B68u3eThHKPGPFAZCQN6sAz6vzHsYHF8t4Q6VcXxFM")
	assert.NoError(t, err, "Failed to decode validator2ID")

	hostID, err := peer.Decode("QmYwAPJzv5CZsnAztbCQdThNzNhnVZaopBRh3HFD1Fvfn7")
	assert.NoError(t, err, "Failed to decode hostID")

	// Add validators to the ValidatorSet
	validators.AddValidator(validator1ID, pk1, privKey1)
	validators.AddValidator(validator2ID, pk2, privKey2)
	validators.AddValidator(hostID, pk3, privKey3)

	// Log current validators
	printValidators(t, validators)

	// Elect a leader (set to host node for predictable behavior)
	validators.SetLeader(hostID)

	// Set quorum size for testing
	validators.SetQuorumThreshold(2)

	// Create a unique temporary directory for MDBX databases
	tempDir := t.TempDir()

	// Define MDBX configuration with correct units
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "consensus",
				Path:            tempDir + "/consensus.mdbx",
				MaxReaders:      4096,
				MaxSize:         1024, // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
		},
	}

	// Create storage manager
	storageMgr, err := storage.NewManager(ctx, mdbxConfig)
	assert.NoError(t, err, "Failed to create storage manager")

	// Create a mock P2PNetwork with HostID as hostID
	mockP2P := networking.NewMockP2PNetwork(hostID, logger)

	// Create a new consensus protocol using the extended constructor
	finalizer := NewMockBlockFinalizer()
	consensus, err := NewConsensusProtocolExtended(ctx, validators, storageMgr, logger, mockP2P, finalizer)
	assert.NoError(t, err, "Failed to create consensus protocol")

	// Brief sleep to allow the Start goroutine to subscribe
	time.Sleep(50 * time.Millisecond)

	return consensus, ctx, validators, storageMgr, hostID
}

// TestGetValidator ensures that all validators can be retrieved correctly.
func TestGetValidator(t *testing.T) {
	_, _, validators, _, hostID := setupTestConsensus(t)

	// Define peer IDs
	validator1ID, err := peer.Decode("QmWmyoCVmCuB8h5eX7ZpZ1eZC8F3JY4A9uYjnrF1i5jdbY")
	assert.NoError(t, err, "Failed to decode validator1ID")

	validator2ID, err := peer.Decode("QmU5B68u3eThHKPGPFAZCQN6sAz6vzHsYHF8t4Q6VcXxFM")
	assert.NoError(t, err, "Failed to decode validator2ID")

	// Retrieve validators
	v1 := validators.GetValidator(validator1ID)
	assert.NotNil(t, v1, "Validator1 should exist")

	v2 := validators.GetValidator(validator2ID)
	assert.NotNil(t, v2, "Validator2 should exist")

	hostValidator := validators.GetValidator(hostID)
	assert.NotNil(t, hostValidator, "Host validator should exist")
}

// TestAutoApproval verifies that auto-approval is functioning correctly.
func TestAutoApproval(t *testing.T) {
	consensus, _, _, _, hostID := setupTestConsensus(t)

	// Propose a new block
	blockData := []byte("auto-approve test block")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")

	// Wait for two messages: proposal + auto-approve
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 2, 1*time.Second)

	// Validate that the auto-approve message exists
	broadcastedMessages := mockP2P.GetBroadcastedMessages()
	assert.Len(t, broadcastedMessages, 2, "Two messages should be broadcasted (proposal + auto-approve)")

	// Deserialize messages
	msg1, err := messages.DeserializeConsensusMessage(broadcastedMessages[0])
	assert.NoError(t, err, "Failed to deserialize first message")
	msg2, err := messages.DeserializeConsensusMessage(broadcastedMessages[1])
	assert.NoError(t, err, "Failed to deserialize second message")

	// Verify message types
	assert.Equal(t, messages.ProposalMessage, msg1.Type, "First message should be ProposalMessage")
	assert.Equal(t, messages.ApprovalMessage, msg2.Type, "Second message should be ApprovalMessage")

	// Verify auto-approve is from host node
	assert.Equal(t, hostID, msg2.ValidatorID, "Auto-approve should be from host node")
}

// TestBlockProposal tests the block proposal functionality.
func TestBlockProposal(t *testing.T) {
	consensus, _, validators, _, hostID := setupTestConsensus(t)

	// Propose a new block with unique data
	blockData := []byte("unique block data for TestBlockProposal")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")

	// Wait for two messages: proposal + auto-approve
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 2, 1*time.Second)

	// Validate the proposal state
	blockHash := HashData(blockData)
	assert.True(t, consensus.state.HasProposal(blockHash), "Proposal should be present in state")

	// Optionally, verify the proposal details
	proposal := consensus.state.GetProposal(blockHash)
	assert.NotNil(t, proposal, "Proposal should not be nil")
	assert.Equal(t, messages.ProposalMessage, proposal.Type, "Message type should be ProposalMessage")
	assert.Equal(t, validators.CurrentLeader().ID, proposal.ProposerID, "Proposer ID should match leader ID")

	// Verify that two messages were broadcasted: proposal + auto-approve
	broadcastedMessages := mockP2P.GetBroadcastedMessages()
	assert.Len(t, broadcastedMessages, 2, "Two messages should be broadcasted (proposal + auto-approve)")

	// Deserialize the first broadcasted message (proposal)
	broadcastedMsg1, err := messages.DeserializeConsensusMessage(broadcastedMessages[0])
	assert.NoError(t, err, "Failed to deserialize first broadcasted message")
	assert.Equal(t, messages.ProposalMessage, broadcastedMsg1.Type, "First broadcasted message type should be ProposalMessage")
	assert.Equal(t, validators.CurrentLeader().ID, broadcastedMsg1.ProposerID, "Proposer ID should match")
	assert.Equal(t, blockHash, broadcastedMsg1.BlockHash, "Block hash should match")
	assert.Equal(t, blockData, broadcastedMsg1.BlockData, "Block data should match")
	assert.NotNil(t, broadcastedMsg1.Signature, "Proposal message should have a signature")

	// Deserialize the second broadcasted message (auto-approve)
	broadcastedMsg2, err := messages.DeserializeConsensusMessage(broadcastedMessages[1])
	assert.NoError(t, err, "Failed to deserialize second broadcasted message")
	assert.Equal(t, messages.ApprovalMessage, broadcastedMsg2.Type, "Second broadcasted message type should be ApprovalMessage")
	assert.Equal(t, hostID, broadcastedMsg2.ValidatorID, "Validator ID should match HostID")
	assert.Equal(t, blockHash, broadcastedMsg2.BlockHash, "Block hash should match")
	assert.Nil(t, broadcastedMsg2.BlockData, "Block data should be nil for ApprovalMessage")
	assert.NotNil(t, broadcastedMsg2.Signature, "Approval message should have a signature")
}

// TestApproval tests the approval functionality for a block.
func TestApproval(t *testing.T) {
	consensus, _, validators, _, _ := setupTestConsensus(t)

	// Propose a new block with unique data
	blockData := []byte("unique block data for TestApproval")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")
	blockHash := HashData(blockData)

	// Wait for two messages: proposal + auto-approve
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 2, 1*time.Second)

	// Approve the proposed block by validator-2
	approverID := "QmU5B68u3eThHKPGPFAZCQN6sAz6vzHsYHF8t4Q6VcXxFM"
	approverPeerID, err := peer.Decode(approverID)
	assert.NoError(t, err, "Failed to decode approverPeerID")

	approver := validators.GetValidator(approverPeerID)
	assert.NotNil(t, approver, "Approver validator should exist")

	err = consensus.ApproveProposal(blockHash, approver.ID)
	assert.NoError(t, err, "Approval by validator-2 failed")

	// Wait for three messages: proposal + auto-approve + approval
	waitForBroadcastedMessages(t, mockP2P, 3, 1*time.Second)

	// Now, it should reach quorum (2 approvals: auto-approve from host + approval by validator-2)
	assert.True(t, consensus.state.HasReachedQuorum(blockHash, validators.QuorumSize()), "Block should reach quorum")

	// Verify that three messages were broadcasted: proposal + auto-approve + approval
	broadcastedMessages := mockP2P.GetBroadcastedMessages()
	assert.Len(t, broadcastedMessages, 3, "Three messages should be broadcasted (proposal + auto-approve + approval)")

	// Deserialize the third broadcasted message (approval by validator-2)
	broadcastedMsg3, err := messages.DeserializeConsensusMessage(broadcastedMessages[2])
	assert.NoError(t, err, "Failed to deserialize third broadcasted message")
	assert.Equal(t, messages.ApprovalMessage, broadcastedMsg3.Type, "Third broadcasted message type should be ApprovalMessage")
	assert.Equal(t, approver.ID, broadcastedMsg3.ValidatorID, "Validator ID should match approver ID")
	assert.Equal(t, blockHash, broadcastedMsg3.BlockHash, "Block hash should match")
	assert.Nil(t, broadcastedMsg3.BlockData, "Block data should be nil for ApprovalMessage")
	assert.NotNil(t, broadcastedMsg3.Signature, "Approval message should have a signature")
}

// TestFinalizeBlock tests finalizing a block.
func TestFinalizeBlock(t *testing.T) {
	consensus, _, validators, _, _ := setupTestConsensus(t)

	// Log all validators
	printValidators(t, validators)

	// Propose a new block with unique data
	blockData := []byte("unique block data for TestFinalizeBlock")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")
	blockHash := HashData(blockData)

	// Approve the proposed block by validator-2
	approverID := "QmU5B68u3eThHKPGPFAZCQN6sAz6vzHsYHF8t4Q6VcXxFM"
	approverPeerID, err := peer.Decode(approverID)
	assert.NoError(t, err, "Failed to decode approverPeerID")

	approver := validators.GetValidator(approverPeerID)
	assert.NotNil(t, approver, "Approver validator should exist")

	err = consensus.ApproveProposal(blockHash, approver.ID)
	assert.NoError(t, err, "Approval by validator-2 failed")

	// Wait for three messages: proposal + auto-approve + approval
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 3, 1*time.Second)

	// Now, it should reach quorum (2 approvals: auto-approve from host + approval by validator-2)
	assert.True(t, consensus.state.HasReachedQuorum(blockHash, validators.QuorumSize()), "Block should reach quorum")

	// Finalize the block
	err = consensus.FinalizeBlock(blockHash)
	assert.NoError(t, err, "Block finalization failed")

	// Wait for four messages: proposal + auto-approve + approval + finalization
	waitForBroadcastedMessages(t, mockP2P, 4, 1*time.Second)

	// Check finalization state
	assert.True(t, consensus.state.IsFinalized(blockHash), "Block should be finalized")

	// Verify that four messages were broadcasted: proposal + auto-approve + approval + finalization
	broadcastedMessages := mockP2P.GetBroadcastedMessages()
	assert.Len(t, broadcastedMessages, 4, "Four messages should be broadcasted (proposal + auto-approve + approval + finalization)")

	// Deserialize the fourth broadcasted message (finalization)
	finalizationMsg, err := messages.DeserializeConsensusMessage(broadcastedMessages[3])
	assert.NoError(t, err, "Failed to deserialize finalization message")
	assert.Equal(t, messages.FinalizationMessage, finalizationMsg.Type, "Finalization message type should be FinalizationMessage")
	assert.Equal(t, blockHash, finalizationMsg.BlockHash, "Block hash should match")
	assert.Nil(t, finalizationMsg.Signature, "Finalization message should not have a signature")
}

// TestSignatureVerification tests BLS signature generation and verification.
func TestSignatureVerification(t *testing.T) {
	// Initialize BLS library for testing
	encryption.InitBLS()

	// Generate BLS keys
	privateKey, publicKey, err := encryption.GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys")

	// Sign some data
	data := []byte("test data")
	signature, err := encryption.SignData(data, privateKey)
	assert.NoError(t, err, "Failed to sign data")
	assert.NotNil(t, signature, "Signature should not be nil")

	// Verify the signature
	valid := encryption.VerifySignature(data, signature, publicKey)
	assert.True(t, valid, "Signature should be valid")
}
