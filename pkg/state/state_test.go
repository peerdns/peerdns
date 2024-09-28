// pkg/state/state_test.go
package state

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestStateManager(t *testing.T) {
	// Create a unique temporary directory for the test database
	tempDir := t.TempDir()
	err := os.MkdirAll(tempDir, 0700)
	require.NoError(t, err, "Failed to create test directory")

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Define MDBX configuration
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "test-db",
				Path:            tempDir + "/test-db-state.mdbx",
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
	require.NoError(t, err, "Failed to create storage manager")
	t.Cleanup(func() {
		if err := storageMgr.Close(); err != nil {
			t.Fatalf("Failed to close StateManager: %v", err)
		}
	})

	// Retrieve or create a specific DB for testing
	testDB, err := storageMgr.GetDb("test-db")
	if err != nil {
		testDB, err = storageMgr.CreateDb("test-db")
		require.NoError(t, err, "Failed to create test DB")
	}

	// Initialize Logger
	logger := log.New(os.Stdout, "StateTest: ", log.LstdFlags)

	// Initialize StateManager with the test database
	sm := NewStateManager(testDB.(*storage.Db), logger)
	defer sm.Close()

	// Test AddProposal
	proposal := &Proposal{
		ID:        "proposal1",
		BlockHash: []byte{0x1, 0x2, 0x3},
		Content:   []byte("Test Block Content"),
		Timestamp: time.Now().Unix(),
	}
	err = sm.AddProposal(proposal)
	require.NoError(t, err, "Failed to add proposal")

	// Test GetProposal
	retrievedProposal, err := sm.GetProposal("proposal1")
	require.NoError(t, err, "Failed to get proposal")
	require.Equal(t, proposal.ID, retrievedProposal.ID, "Proposal IDs should match")
	require.Equal(t, proposal.BlockHash, retrievedProposal.BlockHash, "BlockHashes should match")
	require.Equal(t, proposal.Content, retrievedProposal.Content, "Contents should match")

	// Test ListProposals
	proposals, err := sm.ListProposals()
	require.NoError(t, err, "Failed to list proposals")
	require.Len(t, proposals, 1, "There should be one proposal")
	require.Equal(t, "proposal1", proposals[0].ID, "Proposal ID should match")

	// Test AddPeer
	peer := &Peer{
		ID:       "peer1",
		Address:  "/ip4/127.0.0.1/tcp/9000",
		JoinedAt: time.Now().Unix(),
	}
	err = sm.AddPeer(peer)
	require.NoError(t, err, "Failed to add peer")

	// Test GetPeer
	retrievedPeer, err := sm.GetPeer("peer1")
	require.NoError(t, err, "Failed to get peer")
	require.Equal(t, peer.ID, retrievedPeer.ID, "Peer IDs should match")
	require.Equal(t, peer.Address, retrievedPeer.Address, "Peer addresses should match")
	require.Nil(t, retrievedPeer.LeftAt, "Peer should not have LeftAt timestamp")

	// Test RemovePeer
	leftAt := time.Now().Unix()
	err = sm.RemovePeer("peer1", leftAt)
	require.NoError(t, err, "Failed to remove peer")

	// Verify peer removal
	removedPeer, err := sm.GetPeer("peer1")
	require.NoError(t, err, "Failed to get removed peer")
	require.NotNil(t, removedPeer.LeftAt, "Peer should have LeftAt timestamp")
	require.Equal(t, leftAt, *removedPeer.LeftAt, "LeftAt timestamps should match")

	// Test ListPeers
	peers, err := sm.ListPeers()
	require.NoError(t, err, "Failed to list peers")
	require.Len(t, peers, 0, "There should be no active peers")

	// Test RecordLatency
	latency := &LatencyRecord{
		PeerID:    "peer2",
		LatencyMs: 150,
		Timestamp: time.Now().Unix(),
	}
	err = sm.RecordLatency(latency)
	require.NoError(t, err, "Failed to record latency")

	// Test GetLatencyRecords
	latencies, err := sm.GetLatencyRecords("peer2")
	require.NoError(t, err, "Failed to get latency records")
	require.Len(t, latencies, 1, "There should be one latency record")
	require.Equal(t, latency.LatencyMs, latencies[0].LatencyMs, "LatencyMs should match")

	// Test AddFinalization
	finalization := &Finalization{
		BlockHash: []byte{0x1, 0x2, 0x3},
		Timestamp: time.Now().Unix(),
	}
	err = sm.AddFinalization(finalization)
	require.NoError(t, err, "Failed to add finalization")

	// Test GetFinalization
	retrievedFinalization, err := sm.GetFinalization([]byte{0x1, 0x2, 0x3})
	require.NoError(t, err, "Failed to get finalization")
	require.Equal(t, finalization.BlockHash, retrievedFinalization.BlockHash, "BlockHashes should match")
	require.Equal(t, finalization.Timestamp, retrievedFinalization.Timestamp, "Timestamps should match")

	// Test ListFinalizations
	finalizations, err := sm.ListFinalizations()
	require.NoError(t, err, "Failed to list finalizations")
	require.Len(t, finalizations, 1, "There should be one finalization")
	require.Equal(t, finalization.BlockHash, finalizations[0].BlockHash, "BlockHashes should match")
}
