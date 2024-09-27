// pkg/consensus/consensus_test.go
package consensus

import (
	"context"
	"github.com/peerdns/peerdns/pkg/storage"
	"log"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func setupTestConsensus() (*ConsensusProtocol, context.Context, *ValidatorSet) {
	logger := log.New(os.Stdout, "ConsensusTest: ", log.LstdFlags)
	ctx := context.Background()

	// Initialize BLS library for testing
	InitBLS()

	// Create validators and elect a leader
	validators := NewValidatorSet(logger)
	_, pk1, _ := GenerateBLSKeys()
	_, pk2, _ := GenerateBLSKeys()
	validators.AddValidator(peer.ID("validator-1"), pk1)
	validators.AddValidator(peer.ID("validator-2"), pk2)
	validators.ElectLeader()

	// Set quorum size for testing
	validators.SetQuorumThreshold(2)

	// Create a mock storage manager
	storage := storage.NewManager(ctx, nil)

	// Create a new consensus protocol
	consensus := NewConsensusProtocol(ctx, validators, storage, logger)

	return consensus, ctx, validators
}

// TestBlockProposal tests the block proposal functionality.
func TestBlockProposal(t *testing.T) {
	consensus, _, validators := setupTestConsensus()

	// Propose a new block
	blockData := []byte("test block data")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")

	// Validate the proposal state
	blockHash := HashData(blockData)
	assert.True(t, consensus.state.HasProposal(blockHash), "Proposal should be present in state")
}

// TestApproval tests the approval functionality for a block.
func TestApproval(t *testing.T) {
	consensus, _, validators := setupTestConsensus()

	// Propose a new block
	blockData := []byte("test block data")
	err := consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Block proposal failed")

	// Approve the proposed block
	blockHash := HashData(blockData)
	err = consensus.ApproveProposal(blockHash)
	assert.NoError(t, err, "Block approval failed")

	// Check if the block has reached quorum
	assert.True(t, consensus.state.HasReachedQuorum(blockHash, validators.QuorumSize()), "Block should reach quorum")
}

// TestFinalizeBlock tests finalizing a block.
func TestFinalizeBlock(t *testing.T) {
	consensus, _, _ := setupTestConsensus()

	// Propose and approve a block
	blockData := []byte("test block data")
	blockHash := HashData(blockData)
	consensus.ProposeBlock(blockData)
	consensus.ApproveProposal(blockHash)

	// Simulate additional approvals to meet quorum
	consensus.state.AddApproval(&ConsensusMessage{BlockHash: blockHash, ValidatorID: peer.ID("validator-2")})

	// Finalize the block
	err := consensus.FinalizeBlock(blockHash)
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
