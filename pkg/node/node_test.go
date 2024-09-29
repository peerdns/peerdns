// pkg/node/node_test.go
package node

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestNode initializes a test node with in-memory configurations.
func setupTestNode(t *testing.T) (*Node, context.Context, *storage.Manager) {
	// Initialize the logger
	err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err, "Failed to initialize logger")

	// Create context
	ctx := context.Background()

	// Initialize BLS library for testing
	err = encryption.InitBLS()
	require.NoError(t, err, "Failed to initialize BLS library")

	// Create a unique temporary directory for MDBX databases
	tempDir := t.TempDir()

	// Define MDBX configuration
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "identity",
				Path:            tempDir + "/identity.mdbx",
				MaxReaders:      4096,
				MaxSize:         1024, // in GB
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB
				FilePermissions: 0600,
			},
			{
				Name:            "chain",
				Path:            tempDir + "/chain.mdbx",
				MaxReaders:      4096,
				MaxSize:         1024,
				MinSize:         1,
				GrowthStep:      4096,
				FilePermissions: 0600,
			},
			{
				Name:            "consensus",
				Path:            tempDir + "/consensus.mdbx",
				MaxReaders:      4096,
				MaxSize:         1024,
				MinSize:         1,
				GrowthStep:      4096,
				FilePermissions: 0600,
			},
		},
	}

	// Create storage manager
	storageMgr, err := storage.NewManager(ctx, mdbxConfig)
	require.NoError(t, err, "Failed to create storage manager")

	// Define networking configuration
	networkingConfig := config.Networking{
		ListenPort:     4000,
		ProtocolID:     "/peerdns/1.0.0",
		BootstrapPeers: []string{},
	}

	// Define sharding configuration
	shardingConfig := config.Sharding{
		ShardCount: 4,
	}

	// Assemble full config
	nodeConfig := config.Config{
		Logger: config.Logger{
			Enabled:     true,
			Environment: "development",
			Level:       "debug",
		},
		Mdbx:       mdbxConfig,
		Networking: networkingConfig,
		Sharding:   shardingConfig,
	}

	// Initialize the node
	nodeInstance, err := NewNode(ctx, nodeConfig)
	require.NoError(t, err, "Failed to initialize node")

	return nodeInstance, ctx, storageMgr
}

// HashData computes the SHA-256 hash of the given data.
func HashData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// TestNodeInitialization tests the initialization of the node.
func TestNodeInitialization(t *testing.T) {
	nodeInstance, _, _ := setupTestNode(t)
	defer nodeInstance.Shutdown()

	assert.NotNil(t, nodeInstance, "Node instance should not be nil")
	assert.NotNil(t, nodeInstance.Validator, "Validator should not be nil")
	assert.NotNil(t, nodeInstance.Consensus, "Consensus module should not be nil")
	assert.NotNil(t, nodeInstance.Network, "Network should not be nil")
}

// TestNodeBlockProposal tests the block proposal functionality.
func TestNodeBlockProposal(t *testing.T) {
	nodeInstance, _, _ := setupTestNode(t)
	defer nodeInstance.Shutdown()

	nodeInstance.Start()
	time.Sleep(100 * time.Millisecond)

	// Propose a new block
	blockData := []byte("Test block data")
	err := nodeInstance.Consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Failed to propose block")

	// Wait for the proposal to be processed
	time.Sleep(500 * time.Millisecond)

	blockHash := HashData(blockData)

	// Verify that the proposal is in the state
	assert.True(t, nodeInstance.Consensus.state.HasProposal(blockHash), "Proposal should be present in state")
}

// TestNodeBlockApproval tests the approval functionality for a block.
func TestNodeBlockApproval(t *testing.T) {
	nodeInstance, _, _ := setupTestNode(t)
	defer nodeInstance.Shutdown()

	nodeInstance.Start()
	time.Sleep(100 * time.Millisecond)

	// Propose a new block
	blockData := []byte("Test block data for approval")
	err := nodeInstance.Consensus.ProposeBlock(blockData)
	require.NoError(t, err, "Failed to propose block")

	// Wait for proposal to be processed
	time.Sleep(500 * time.Millisecond)

	blockHash := HashData(blockData)

	// Approve the proposal by the validator
	validatorID := nodeInstance.Validator.DID.ID // Using the node's own validator
	err = nodeInstance.Consensus.ApproveProposal(blockHash, peer.ID(validatorID))
	require.NoError(t, err, "Failed to approve proposal")

	// Wait for approval to be processed
	time.Sleep(500 * time.Millisecond)

	// Check if the proposal has the approval
	approvalCount := nodeInstance.Consensus.state.GetApprovalCount(blockHash)
	assert.Equal(t, 1, approvalCount, "Approval count should be 1")
}

// TestNodeBlockFinalization tests the block finalization process.
func TestNodeBlockFinalization(t *testing.T) {
	nodeInstance, _, _ := setupTestNode(t)
	defer nodeInstance.Shutdown()

	nodeInstance.Start()
	time.Sleep(100 * time.Millisecond)

	// Propose a new block
	blockData := []byte("Test block data for finalization")
	err := nodeInstance.Consensus.ProposeBlock(blockData)
	require.NoError(t, err, "Failed to propose block")

	// Wait for proposal to be processed
	time.Sleep(500 * time.Millisecond)

	blockHash := HashData(blockData)

	// Approve the proposal
	validatorID := nodeInstance.Validator.DID.ID // Using the node's own validator
	err = nodeInstance.Consensus.ApproveProposal(blockHash, peer.ID(validatorID))
	require.NoError(t, err, "Failed to approve proposal")

	// Wait for approval to be processed
	time.Sleep(500 * time.Millisecond)

	// Check if quorum is reached
	assert.True(t, nodeInstance.Consensus.state.HasReachedQuorum(blockHash), "Quorum should be reached")

	// Finalize the block
	err = nodeInstance.Consensus.FinalizeBlock(blockHash)
	require.NoError(t, err, "Failed to finalize block")

	// Check if block is finalized
	assert.True(t, nodeInstance.Consensus.state.IsFinalized(blockHash), "Block should be finalized")
}
