package consensus

import (
	"context"
	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/networking"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/storage"
)

// BenchmarkConsensusProtocol benchmarks the performance of the SHPoNU Consensus Protocol.
func BenchmarkConsensusProtocol(b *testing.B) {
	// Open a log file
	logFile, err := os.Create("benchmark.log")
	if err != nil {
		b.Fatalf("Failed to create log file: %v", err)
	}

	// Ensure the file is closed after the benchmark
	b.Cleanup(func() {
		if err := logFile.Close(); err != nil {
			b.Errorf("Failed to close log file: %v", err)
		}
	})

	// Initialize logger to write to the file
	logger := log.New(logFile, "ConsensusBenchmark: ", log.LstdFlags)

	ctx := context.Background()

	// Initialize BLS library for testing
	encryption.InitBLS()

	// Create validators and elect a leader
	validators := NewValidatorSet(logger)
	privKey1, pk1, err := encryption.GenerateBLSKeys()
	if err != nil {
		b.Fatalf("Failed to generate BLS keys for validator 1: %v", err)
	}
	privKey2, pk2, err := encryption.GenerateBLSKeys()
	if err != nil {
		b.Fatalf("Failed to generate BLS keys for validator 2: %v", err)
	}

	validators.AddValidator(peer.ID("validator-1"), pk1, privKey1)
	validators.AddValidator(peer.ID("validator-2"), pk2, privKey2)
	validators.ElectLeader()

	// Set quorum size for benchmarking
	validators.SetQuorumThreshold(2)

	// Create a temporary directory for MDBX databases
	tempDir := "./benchmark_test_data"
	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		b.Fatalf("Failed to create temporary directory for MDBX: %v", err)
	}

	// Ensure cleanup after benchmark
	b.Cleanup(func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			b.Errorf("Failed to remove temporary directory: %v", err)
		}
	})

	// Define MDBX configuration with appropriate units
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "consensus",
				Path:            tempDir + "/consensus.mdbx",
				MaxReaders:      4096,
				MaxSize:         1024, // in GB for benchmarking purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // in bytes
				FilePermissions: 0600,
			},
		},
	}

	// Create storage manager
	storageMgr, err := storage.NewManager(ctx, mdbxConfig)
	if err != nil {
		b.Fatalf("Failed to create storage manager: %v", err)
	}

	// Create a mock P2PNetwork with HostID as "mock-host"
	mockHostID := peer.ID("mock-host")
	mockP2P := networking.NewMockP2PNetwork(mockHostID, logger)

	// Create a new consensus protocol using the extended constructor
	finalizer := NewMockBlockFinalizer()
	consensus, err := NewConsensusProtocolExtended(ctx, validators, storageMgr, logger, mockP2P, finalizer)
	if err != nil {
		b.Fatalf("Failed to create consensus protocol: %v", err)
	}

	// Ensure storage manager is closed after benchmark
	b.Cleanup(func() {
		err := storageMgr.Close()
		if err != nil {
			b.Errorf("Failed to close storage manager: %v", err)
		}
	})

	// Pre-generate block data to reuse across iterations
	blockData := []byte("benchmark test block data")
	blockHash := HashData(blockData)

	// Reset the timer to exclude setup time from benchmark measurements
	b.ResetTimer()
	b.ReportAllocs()

	// Use a WaitGroup to manage concurrency
	var wg sync.WaitGroup

	// Define the number of concurrent operations (e.g., goroutines)
	concurrency := 10 // Adjust based on your system's capabilities

	// Define the number of iterations per goroutine
	iterations := b.N / concurrency
	if iterations == 0 {
		iterations = 1
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Propose a new block
				err := consensus.ProposeBlock(blockData)
				if err != nil && err != ErrNotLeader {
					// Only proceed if the proposer is the leader
					// Non-leaders will return ErrNotLeader
					b.Errorf("Goroutine %d: Block proposal failed: %v", goroutineID, err)
					continue
				}

				// If the proposer is not the leader, skip approval
				if err == ErrNotLeader {
					continue
				}

				// Approve the proposed block by leader
				leader := validators.CurrentLeader()
				err = consensus.ApproveProposal(blockHash, leader.ID)
				if err != nil {
					b.Errorf("Goroutine %d: Block approval by leader failed: %v", goroutineID, err)
					continue
				}

				// Approve the block by another validator to meet quorum
				approver := validators.GetValidator(peer.ID("validator-2"))
				if approver == nil {
					b.Errorf("Goroutine %d: Approver validator does not exist", goroutineID)
					continue
				}
				err = consensus.ApproveProposal(blockHash, approver.ID)
				if err != nil {
					b.Errorf("Goroutine %d: Approval by validator-2 failed: %v", goroutineID, err)
					continue
				}

				// Finalize the block
				err = consensus.FinalizeBlock(blockHash)
				if err != nil && err != ErrQuorumNotReached {
					b.Errorf("Goroutine %d: Block finalization failed: %v", goroutineID, err)
					continue
				}
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}
