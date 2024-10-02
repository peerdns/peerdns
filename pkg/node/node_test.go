// pkg/node/node_test.go
package node

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/consensus"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
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

	// Create a unique temporary directory for MDBX databases
	tempDir := t.TempDir()

	// Define MDBX configuration
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "identity",
				Path:            tempDir + "/node-identity.mdbx",
				MaxReaders:      128,
				MaxSize:         1,    // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0700,
			},
			{
				Name:            "chain",
				Path:            tempDir + "/node-chain.mdbx",
				MaxReaders:      128,
				MaxSize:         1,    // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
			{
				Name:            "consensus",
				Path:            tempDir + "/node-consensus.mdbx",
				MaxReaders:      128,
				MaxSize:         1,    // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
		},
	}

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
	nodeInstance, err := NewNode(ctx, nodeConfig, logger.G())
	require.NoError(t, err, "Failed to initialize node")

	// Set quorum threshold for the validator set (e.g., 1 for tests)
	nodeInstance.Validator.ValidatorSet.SetQuorumThreshold(1)

	return nodeInstance, ctx, nodeInstance.GetStorageManager()
}

// TestNodeInitialization tests the initialization of the node.
func TestNodeInitialization(t *testing.T) {
	nodeInstance, _, storageMgr := setupTestNode(t)
	defer func() {
		nodeInstance.Shutdown()
		err := storageMgr.Close()
		require.NoError(t, err, "Failed to close storage manager")
	}()

	assert.NotNil(t, nodeInstance, "Node instance should not be nil")
	assert.NotNil(t, nodeInstance.Validator, "Validator should not be nil")
	assert.NotNil(t, nodeInstance.Consensus, "Consensus module should not be nil")
	assert.NotNil(t, nodeInstance.Network, "Network should not be nil")
}

// TestNodeBlockProposal tests the block proposal functionality.
func TestNodeBlockProposal(t *testing.T) {
	nodeInstance, _, storageMgr := setupTestNode(t)
	defer func() {
		nodeInstance.Shutdown()
		err := storageMgr.Close()
		require.NoError(t, err, "Failed to close storage manager")
	}()

	nodeInstance.Start()
	time.Sleep(100 * time.Millisecond)

	// Propose a new block
	blockData := []byte("Test block data")
	err := nodeInstance.Consensus.ProposeBlock(blockData)
	assert.NoError(t, err, "Failed to propose block")

	// Wait for the proposal to be processed
	time.Sleep(500 * time.Millisecond)

	blockHash := consensus.HashData(blockData)

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

	blockHash := consensus.HashData(blockData)

	// Approve the proposal by the validator
	validatorID := peer.ID(nodeInstance.Validator.DID.ID) // Using the node's own validator
	err = nodeInstance.Consensus.ApproveProposal(blockHash, validatorID)
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

	blockHash := consensus.HashData(blockData)

	// Approve the proposal
	validatorID := peer.ID(nodeInstance.Validator.DID.ID) // Using the node's own validator
	err = nodeInstance.Consensus.ApproveProposal(blockHash, validatorID)
	require.NoError(t, err, "Failed to approve proposal")

	// Wait for approval to be processed
	time.Sleep(500 * time.Millisecond)

	// Get the quorum size from the validator set
	quorumSize := nodeInstance.Validator.ValidatorSet.QuorumSize()

	// Check if quorum is reached
	assert.True(t, nodeInstance.Consensus.state.HasReachedQuorum(blockHash, quorumSize), "Quorum should be reached")

	// Finalize the block
	err = nodeInstance.Consensus.FinalizeBlock(blockHash)
	require.NoError(t, err, "Failed to finalize block")

	// Check if block is finalized
	assert.True(t, nodeInstance.Consensus.state.IsFinalized(blockHash), "Block should be finalized")
}
