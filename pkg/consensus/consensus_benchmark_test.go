package consensus

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/networking"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

// setupBenchmarkConsensus initializes the ConsensusProtocol with a mock P2P network.
// It accepts a testing.TB interface to work with both tests and benchmarks.
func setupBenchmarkConsensus(tb testing.TB) (*Protocol, context.Context, *ValidatorSet, *storage.Manager, peer.ID, peer.ID) {
	// Initialize the test environment.
	gLog, aState := setupTestEnvironment(tb, "error")
	require.NotNil(tb, gLog)
	require.NotNil(tb, aState)

	ctx := context.Background()

	// Create a ValidatorSet with the correct logger.
	validators := NewValidatorSet(gLog)

	// Create validator accounts.
	hostAccount, err := aState.Create("validator_1", "Benchmark Consensus Host Validator", false)
	require.NoError(tb, err)
	require.NotNil(tb, hostAccount)

	account2, err := aState.Create("validator_2", "Benchmark Consensus 2nd Validator", false)
	require.NoError(tb, err)
	require.NotNil(tb, account2)

	account3, err := aState.Create("validator_3", "Benchmark Consensus 3rd Validator", false)
	require.NoError(tb, err)
	require.NotNil(tb, account3)

	// Add validators to the ValidatorSet.
	validators.AddValidator(hostAccount)
	validators.AddValidator(account2)
	validators.AddValidator(account3)

	// Log current validators.
	// printValidators(tb, validators)

	// Initialize a mock metrics.Collector.
	/*	mockCollector := metrics.NewCollector(metrics.Metrics{
		BandwidthUsage: rand.Float64() * 100.0, // 0.0 to 100.0
		Computational:  rand.Float64() * 100.0, // 0.0 to 100.0
		Storage:        rand.Float64() * 100.0, // 0.0 to 100.0
		Uptime:         rand.Float64() * 100.0, // 0.0 to 100.0
		Responsiveness: rand.Float64() * 100.0, // 0.0 to 100.0
		Reliability:    rand.Float64() * 100.0, // 0.0 to 100.0
	})*/

	// Elect a leader by passing the mock collector.
	validators.SetLeader(hostAccount.PeerID)

	// Set quorum size for benchmarking.
	validators.SetQuorumThreshold(2)

	// Create a unique temporary directory for MDBX databases.
	tempDir := tb.TempDir()

	// Define MDBX configuration with correct units.
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "consensus",
				Path:            fmt.Sprintf("%s/consensus.mdbx", tempDir),
				MaxReaders:      4096,
				MaxSize:         1024, // in GB for benchmarking purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // in bytes
				FilePermissions: 0600,
			},
		},
	}

	// Create storage manager.
	storageMgr, err := storage.NewManager(ctx, mdbxConfig)
	assert.NoError(tb, err, "Failed to create storage manager")

	// Create a mock P2PNetwork with HostID as hostID.
	mockP2P := networking.NewMockP2PNetwork(hostAccount.PeerID, gLog)

	// Create a new consensus protocol using the extended constructor.
	finalizer := NewMockBlockFinalizer()
	consensus, err := NewProtocol(ctx, validators, storageMgr, gLog, mockP2P, finalizer)
	assert.NoError(tb, err, "Failed to create consensus protocol")

	return consensus, ctx, validators, storageMgr, hostAccount.PeerID, account2.PeerID
}

// BenchmarkConsensusProtocol benchmarks the performance of the SHPoNU Consensus Protocol.
func BenchmarkConsensusProtocol(b *testing.B) {
	// Initialize the consensus protocol and related components.
	consensus, _, validators, storageMgr, _, account2ID := setupBenchmarkConsensus(b)

	// Ensure storage manager is closed after benchmark.
	b.Cleanup(func() {
		err := storageMgr.Close()
		if err != nil {
			b.Errorf("Failed to close storage manager: %v", err)
		}
	})

	// Reset the timer to exclude setup time from benchmark measurements.
	b.ResetTimer()
	b.ReportAllocs()

	// Use a WaitGroup to manage concurrency.
	var wg sync.WaitGroup

	// Define the number of concurrent operations (e.g., goroutines).
	concurrency := 10 // Adjust based on your system's capabilities.

	// Calculate iterations per goroutine to cover all b.N operations.
	iterationsPerGoroutine := b.N / concurrency
	extraIterations := b.N % concurrency

	// Function to perform benchmark operations.
	benchmarkWorker := func(goroutineID, iterations int) {
		defer wg.Done()
		for j := 0; j < iterations; j++ {
			// Generate unique block data for each iteration.
			blockData := []byte(fmt.Sprintf("benchmark test block data goroutine %d iteration %d", goroutineID, j))
			blockHash := types.HashData(blockData)

			// Propose a new block.
			err := consensus.ProposeBlock(blockData)
			if err != nil && !errors.Is(err, ErrNotLeader) {
				// Only proceed if the proposer is the leader.
				// Non-leaders will return ErrNotLeader.
				b.Errorf("Goroutine %d: Block proposal failed: %v", goroutineID, err)
				continue
			}

			// If the proposer is not the leader, skip approval.
			if err == ErrNotLeader {
				continue
			}

			// Approve the proposed block by leader.
			leader := validators.CurrentLeader()
			err = consensus.ApproveProposal(blockHash, leader.PeerID())
			if err != nil {
				b.Errorf("Goroutine %d: Block approval by leader failed: %v", goroutineID, err)
				continue
			}

			// Approve the block by another validator to meet quorum.
			approver := validators.GetValidator(account2ID)
			if approver == nil {
				b.Errorf("Goroutine %d: Approver validator does not exist", goroutineID)
				continue
			}
			err = consensus.ApproveProposal(blockHash, approver.PeerID())
			if err != nil {
				b.Errorf("Goroutine %d: Approval by validator-2 failed: %v", goroutineID, err)
				continue
			}

			// Finalize the block.
			err = consensus.FinalizeBlock(blockHash)
			if err != nil && err != ErrQuorumNotReached {
				b.Errorf("Goroutine %d: Block finalization failed: %v", goroutineID, err)
				continue
			}
		}
	}

	// Launch concurrent goroutines.
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go benchmarkWorker(i, iterationsPerGoroutine)
	}

	// Handle any extra iterations if b.N is not perfectly divisible by concurrency.
	if extraIterations > 0 {
		wg.Add(1)
		go benchmarkWorker(concurrency, extraIterations)
	}

	// Wait for all goroutines to finish.
	wg.Wait()
}
