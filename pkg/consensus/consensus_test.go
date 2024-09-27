package consensus

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/storage"
	"log"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func setupTestConsensus(t *testing.T) (*ConsensusProtocol, context.Context, *ValidatorSet, *storage.Manager) {
	logger := log.New(os.Stdout, "ConsensusTest: ", log.LstdFlags)
	ctx := context.Background()

	// Initialize BLS library for testing
	InitBLS()

	// Create validators and elect a leader
	validators := NewValidatorSet(logger)
	privKey1, pk1, err := GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys for validator 1")
	privKey2, pk2, err := GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys for validator 2")

	validators.AddValidator(peer.ID("validator-1"), pk1, privKey1)
	validators.AddValidator(peer.ID("validator-2"), pk2, privKey2)
	validators.ElectLeader()

	// Set quorum size for testing
	validators.SetQuorumThreshold(2)

	// Create a temporary directory for MDBX databases
	tempDir := "./test_data"
	err = os.MkdirAll(tempDir, 0755)
	assert.NoError(t, err, "Failed to create temporary directory for MDBX")

	// Ensure cleanup after tests
	t.Cleanup(func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Errorf("Failed to remove temporary directory: %v", err)
		}
	})

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
				GrowthStep:      4096, // 10 MB
				FilePermissions: 0600,
			},
		},
	}

	// Create storage manager
	storageMgr, err := storage.NewManager(ctx, mdbxConfig)
	assert.NoError(t, err, "Failed to create storage manager")

	// Create a new consensus protocol
	consensus, err := NewConsensusProtocol(ctx, validators, storageMgr, logger)
	assert.NoError(t, err, "Failed to create consensus protocol")

	return consensus, ctx, validators, storageMgr
}

// TestBlockProposal tests the block proposal functionality.
func TestBlockProposal(t *testing.T) {
	consensus, _, validators, _ := setupTestConsensus(t)

	// Propose a new block
	blockData := []byte("test block data")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")

	// Validate the proposal state
	blockHash := HashData(blockData)
	assert.True(t, consensus.state.HasProposal(blockHash), "Proposal should be present in state")

	// Optionally, verify the proposal details
	proposal := consensus.state.GetProposal(blockHash)
	assert.NotNil(t, proposal, "Proposal should not be nil")
	assert.Equal(t, ProposalMessage, proposal.Type, "Message type should be ProposalMessage")
	assert.Equal(t, validators.CurrentLeader().ID, proposal.ProposerID, "Proposer ID should match leader ID")
}

// TestApproval tests the approval functionality for a block.
func TestApproval(t *testing.T) {
	consensus, _, validators, _ := setupTestConsensus(t)

	// Propose a new block
	blockData := []byte("test block data")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")

	// Get block hash
	blockHash := HashData(blockData)

	// Approve the proposed block by leader
	leader := validators.CurrentLeader()
	err = consensus.ApproveProposal(blockHash, leader.ID)
	assert.NoError(t, err, "Block approval by leader failed")

	// Check if the block has reached quorum (needs two approvals)
	// Since quorum is set to 2, but only one approval so far, it should not have reached quorum
	assert.False(t, consensus.state.HasReachedQuorum(blockHash, validators.QuorumSize()), "Block should not reach quorum with one approval")

	// Approve the proposed block by another validator
	approver := validators.GetValidator(peer.ID("validator-2"))
	assert.NotNil(t, approver, "Approver validator should exist")
	err = consensus.ApproveProposal(blockHash, approver.ID)
	assert.NoError(t, err, "Approval by validator-2 failed")

	// Now, it should reach quorum
	assert.True(t, consensus.state.HasReachedQuorum(blockHash, validators.QuorumSize()), "Block should reach quorum")
}

// TestFinalizeBlock tests finalizing a block.
func TestFinalizeBlock(t *testing.T) {
	consensus, _, validators, _ := setupTestConsensus(t)

	// Propose a new block
	blockData := []byte("test block data")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")
	blockHash := HashData(blockData)

	// Approve the proposed block by leader
	leader := validators.CurrentLeader()
	err = consensus.ApproveProposal(blockHash, leader.ID)
	assert.NoError(t, err, "Block approval by leader failed")

	// Approve the block by another validator to reach quorum
	approver := validators.GetValidator(peer.ID("validator-2"))
	assert.NotNil(t, approver, "Approver validator should exist")
	err = consensus.ApproveProposal(blockHash, approver.ID)
	assert.NoError(t, err, "Approval by validator-2 failed")

	// Finalize the block
	err = consensus.FinalizeBlock(blockHash)
	assert.NoError(t, err, "Block finalization failed")

	// Check finalization state
	assert.True(t, consensus.state.IsFinalized(blockHash), "Block should be finalized")
}

// TestSignatureVerification tests BLS signature generation and verification.
func TestSignatureVerification(t *testing.T) {
	// Initialize BLS library for testing
	InitBLS()

	// Generate BLS keys
	privateKey, publicKey, err := GenerateBLSKeys()
	assert.NoError(t, err, "Failed to generate BLS keys")

	// Sign some data
	data := []byte("test data")
	signature, err := SignData(data, privateKey)
	assert.NoError(t, err, "Failed to sign data")

	// Verify the signature
	valid := VerifySignature(data, signature, publicKey)
	assert.True(t, valid, "Signature should be valid")
}
