// pkg/consensus/consensus_test.go
package consensus

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
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
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-mockP2P.BroadcastCh():
			t.Logf("Received broadcasted message #%d", mockP2P.GetBroadcastedCount())
			if mockP2P.GetBroadcastedCount() >= expected {
				return
			}
		case <-timer.C:
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
func setupTestConsensus(t *testing.T) (*Protocol, context.Context, *ValidatorSet, *storage.Manager, peer.ID) {
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
	consensus, err := NewProtocol(ctx, validators, storageMgr, logger, mockP2P, finalizer)
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
	blockHash := HashData(blockData)

	// Wait for at least one message: proposal
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 1, 2*time.Second)

	// Validate that the auto-approve message exists in state
	assert.True(t, consensus.state.HasProposal(blockHash), "Proposal should be present in state")

	// Verify that at least one message was broadcasted (proposal)
	broadcastedMessages := mockP2P.GetBroadcastedMessages()
	assert.Len(t, broadcastedMessages, 1, "One message should be broadcasted (proposal)")

	// Deserialize messages and check the type
	msg, err := packet.DeserializeConsensusMessage(broadcastedMessages[0].Data)
	assert.NoError(t, err, "Failed to deserialize broadcasted message")
	assert.Equal(t, packet.ProposalMessage, msg.Type, "Broadcasted message should be ProposalMessage")
	assert.Equal(t, hostID, msg.ProposerID, "Proposer ID should match host node")
}

// TestBlockProposal tests the block proposal functionality.
func TestBlockProposal(t *testing.T) {
	consensus, _, validators, _, _ := setupTestConsensus(t)

	// Propose a new block with unique data
	blockData := []byte("unique block data for TestBlockProposal")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")
	blockHash := HashData(blockData)

	// Wait for at least one message: proposal
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 1, 2*time.Second)

	// Validate the proposal state
	assert.True(t, consensus.state.HasProposal(blockHash), "Proposal should be present in state")

	// Verify that at least one message was broadcasted (proposal)
	broadcastedMessages := mockP2P.GetBroadcastedMessages()
	assert.Len(t, broadcastedMessages, 1, "One message should be broadcasted (proposal)")

	// Deserialize the first broadcasted message (proposal)
	broadcastedMsg, err := packet.DeserializeConsensusMessage(broadcastedMessages[0].Data)
	assert.NoError(t, err, "Failed to deserialize first broadcasted message")
	assert.Equal(t, packet.ProposalMessage, broadcastedMsg.Type, "First broadcasted message type should be ProposalMessage")
	assert.Equal(t, validators.CurrentLeader().ID, broadcastedMsg.ProposerID, "Proposer ID should match leader ID")
	assert.Equal(t, blockHash, broadcastedMsg.BlockHash, "Block hash should match")
	assert.Equal(t, blockData, broadcastedMsg.BlockData, "Block data should match")
	assert.NotNil(t, broadcastedMsg.Signature, "Proposal message should have a signature")
}

// TestApproval tests the approval functionality for a block.
func TestApproval(t *testing.T) {
	consensus, _, validators, _, _ := setupTestConsensus(t)

	// Propose a new block with unique data
	blockData := []byte("unique block data for TestMultipleApprovals")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")
	blockHash := HashData(blockData)

	// Wait for the proposal message
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 1, 2*time.Second)

	// Approve the block by validator-2
	approverID := "QmU5B68u3eThHKPGPFAZCQN6sAz6vzHsYHF8t4Q6VcXxFM"
	approverPeerID, err := peer.Decode(approverID)
	assert.NoError(t, err, "Failed to decode approverPeerID")

	approver := validators.GetValidator(approverPeerID)
	assert.NotNil(t, approver, "Approver validator should exist")

	err = consensus.ApproveProposal(blockHash, approver.ID)
	assert.NoError(t, err, "Approval by validator-2 failed")

	// Approve the block by validator-1
	approverID2 := "QmWmyoCVmCuB8h5eX7ZpZ1eZC8F3JY4A9uYjnrF1i5jdbY"
	approverPeerID2, err := peer.Decode(approverID2)
	assert.NoError(t, err, "Failed to decode approverPeerID2")

	approver2 := validators.GetValidator(approverPeerID2)
	assert.NotNil(t, approver2, "Approver validator 2 should exist")

	err = consensus.ApproveProposal(blockHash, approver2.ID)
	assert.NoError(t, err, "Approval by validator-1 failed")

	// Wait for the approval messages
	waitForBroadcastedMessages(t, mockP2P, 2, 2*time.Second)

	// Ensure quorum is reached with two distinct approvals
	assert.True(t, consensus.state.HasReachedQuorum(blockHash, validators.QuorumSize()), "Block should reach quorum with 2 approvals")
}

// TestFinalizeBlock tests the block finalization process.
func TestFinalizeBlock(t *testing.T) {
	consensus, _, validators, _, _ := setupTestConsensus(t)

	// Log all validators for debugging purposes
	printValidators(t, validators)

	// Propose a new block with unique data
	blockData := []byte("unique block data for TestFinalizeBlock")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")
	blockHash := HashData(blockData)

	// Wait for at least one message: proposal
	mockP2P := consensus.p2pNetwork.(*networking.MockP2PNetwork)
	waitForBroadcastedMessages(t, mockP2P, 1, 2*time.Second)

	// Approve the proposed block by validator-2
	approverID := "QmU5B68u3eThHKPGPFAZCQN6sAz6vzHsYHF8t4Q6VcXxFM"
	approverPeerID, err := peer.Decode(approverID)
	assert.NoError(t, err, "Failed to decode approverPeerID")

	approver := validators.GetValidator(approverPeerID)
	assert.NotNil(t, approver, "Approver validator should exist")

	// Approve the proposal
	err = consensus.ApproveProposal(blockHash, approver.ID)
	assert.NoError(t, err, "Approval by validator-2 failed")

	// Approve the block by validator-1
	approverID2 := "QmWmyoCVmCuB8h5eX7ZpZ1eZC8F3JY4A9uYjnrF1i5jdbY"
	approverPeerID2, err := peer.Decode(approverID2)
	assert.NoError(t, err, "Failed to decode approverPeerID2")

	approver2 := validators.GetValidator(approverPeerID2)
	assert.NotNil(t, approver2, "Approver validator 2 should exist")

	err = consensus.ApproveProposal(blockHash, approver2.ID)
	assert.NoError(t, err, "Approval by validator-1 failed")

	// Wait for the approval messages
	waitForBroadcastedMessages(t, mockP2P, 2, 2*time.Second)

	// Sleep for a short time to ensure the state has been updated before checking
	time.Sleep(100 * time.Millisecond)

	// Check the internal state of approvals for the block
	t.Logf("Internal state: Total approvals for block %x: %d", blockHash, consensus.state.GetApprovalCount(blockHash))

	// Check if quorum is reached with two distinct approvals
	assert.True(t, consensus.state.HasReachedQuorum(blockHash, validators.QuorumSize()), "Block should reach quorum with 2 approvals")

	// Finalize the block
	err = consensus.FinalizeBlock(blockHash)
	assert.NoError(t, err, "Block finalization failed")

	// Wait for at least three messages: proposal + approvals + finalization
	waitForBroadcastedMessages(t, mockP2P, 3, 2*time.Second)

	// Check finalization state
	assert.True(t, consensus.state.IsFinalized(blockHash), "Block should be finalized")
}

// TestSignatureVerification tests BLS signature generation and verification.
func TestSignatureVerification(t *testing.T) {
	// Initialize BLS library for testing
	err := encryption.InitBLS()
	require.NoError(t, err)

	// Generate BLS keys
	privateKey, publicKey, err := encryption.GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys")

	// Sign some data
	data := []byte("test data")
	signature, err := encryption.Sign(data, privateKey)
	assert.NoError(t, err, "Failed to sign data")
	assert.NotNil(t, signature, "Signature should not be nil")

	// Verify the signature
	valid := encryption.Verify(data, signature, publicKey)
	assert.True(t, valid, "Signature should be valid")
}
