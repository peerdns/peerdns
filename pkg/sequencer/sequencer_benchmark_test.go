// sequencer_benchmark_test.go

package sequencer

import (
	"context"
	"crypto/sha256"
	"github.com/peerdns/peerdns/pkg/chain"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/genesis"
	"github.com/peerdns/peerdns/pkg/ledger"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/txpool"
	"github.com/peerdns/peerdns/pkg/types"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// BenchmarkSequencer_ProduceBlock benchmarks the block production process.
func BenchmarkSequencer_ProduceBlock(b *testing.B) {
	ctx := context.Background()

	// Initialize global logger
	gLog, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	if err != nil {
		b.Fatalf("Failed to initialize global logger: %v", err)
	}

	// Setup temporary directory and database
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "blockchain_benchmark.db")
	storageConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "blockchain_benchmark",
				Path:            dbPath,
				MaxReaders:      4096,
				MaxSize:         1,
				MinSize:         1,
				GrowthStep:      4096,
				FilePermissions: 0600,
			},
		},
	}

	// Initialize storage manager and database
	storageManager, err := storage.NewManager(ctx, storageConfig)
	if err != nil {
		b.Fatalf("Failed to create storage manager: %v", err)
	}
	db, err := storageManager.GetDb("blockchain_benchmark")
	if err != nil {
		b.Fatalf("Failed to get database: %v", err)
	}

	// Initialize ledger
	ldgr, err := ledger.NewLedger(ctx, db)
	if err != nil {
		b.Fatalf("Failed to create new ledger: %v", err)
	}

	// Create and add genesis block
	genesisValidator := createAddress("did:peer:genesis-validator")
	genesisConfig := &genesis.Genesis{
		Difficulty: 0,
		Timestamp:  time.Now().Unix(),
		Alloc: map[string]genesis.AllocItem{
			createAddress("did:peer:sender1").Hex():    {Balance: "10000000"},
			createAddress("did:peer:recipient1").Hex(): {Balance: "10000000"},
		},
	}
	genesisBlock, err := genesis.CreateGenesisBlock(genesisValidator, genesisConfig)
	if err != nil {
		b.Fatalf("Failed to create genesis block: %v", err)
	}

	err = ldgr.AddBlock(genesisBlock)
	if err != nil {
		b.Fatalf("Failed to add genesis block to ledger: %v", err)
	}

	// Initialize blockchain
	blockchain, err := chain.NewBlockchain(ctx, gLog, ldgr, genesisConfig)
	if err != nil {
		b.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Initialize TxPool
	txPool := txpool.NewTxPool()

	// Initialize sequencer components
	metrics := &SequencerMetrics{}
	stateMgr := NewStateManager(gLog, metrics)
	blockProducer := NewBlockProducer(txPool, blockchain, gLog, stateMgr)
	sequencer := NewSequencer(ctx, blockProducer, gLog, metrics)

	// Initialize sender's nonce before starting the sequencer
	initialNonce := ldgr.State().GetNonce(createAddress("did:peer:sender1")) + 1
	b.Logf("Initial nonce for sender1: %d", initialNonce) // Log for verification

	// Start the sequencer
	err = sequencer.Start()
	if err != nil {
		b.Fatalf("Sequencer should start without error: %v", err)
	}

	// Wait for the sequencer to reach the Started state
	err = sequencer.stateMgr.WaitForState(SequencerStateType, Started, 500*time.Millisecond)
	if err != nil {
		b.Fatalf("Sequencer should reach Started state: %v", err)
	}

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Use a buffered channel to feed transactions
	txChan := make(chan *types.Transaction, 1000)

	// Start a goroutine to add transactions to the TxPool
	go func() {
		for i := 0; i < b.N; i++ {
			tx := &types.Transaction{
				ID:        sha256.Sum256([]byte("tx" + strconv.Itoa(i+1))),
				Sender:    createAddress("did:peer:sender1"),
				Recipient: createAddress("did:peer:recipient1"),
				Amount:    1000,
				Fee:       10,
				Nonce:     initialNonce + uint64(i), // Assign sequential nonces
				Timestamp: time.Now().Unix(),
				Payload:   []byte("benchmark payload"),
			}

			txChan <- tx
		}
		close(txChan)
	}()

	// Start another goroutine to consume transactions from the channel and add to TxPool
	go func() {
		for tx := range txChan {
			tErr := txPool.AddTransaction(tx)
			if tErr != nil {
				b.Fatalf("Failed to add transaction to TxPool: %v", tErr)
			}
		}
	}()

	// Optionally, monitor the TxPool size or implement synchronization to ensure all transactions are processed
	// For example, wait until TxPool is empty
	for {
		currentSize := txPool.Size()
		if currentSize == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Stop the sequencer
	b.StopTimer() // Stop the timer before cleanup

	time.Sleep(2 * time.Second)

	err = sequencer.Stop()
	if err != nil {
		b.Fatalf("Sequencer should stop without error: %v", err)
	}

	// Wait for the sequencer to reach the Stopped state
	err = sequencer.stateMgr.WaitForState(SequencerStateType, Stopped, 500*time.Millisecond)
	if err != nil {
		b.Fatalf("Sequencer should reach Stopped state: %v", err)
	}
}
