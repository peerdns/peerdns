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

	gLog, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	if err != nil {
		b.Fatalf("Failed to initialize global logger: %v", err)
	}

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

	storageManager, err := storage.NewManager(ctx, storageConfig)
	if err != nil {
		b.Fatalf("Failed to create storage manager: %v", err)
	}
	db, err := storageManager.GetDb("blockchain_benchmark")
	if err != nil {
		b.Fatalf("Failed to get database: %v", err)
	}

	ldgr, err := ledger.NewLedger(ctx, db)
	if err != nil {
		b.Fatalf("Failed to create new ledger: %v", err)
	}

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

	blockchain, err := chain.NewBlockchain(ctx, gLog, ldgr, genesisConfig)
	if err != nil {
		b.Fatalf("Failed to initialize blockchain: %v", err)
	}

	txPool := txpool.NewTxPool() // Use a real TxPool instance
	metrics := &SequencerMetrics{}
	stateMgr := NewStateManager(gLog, metrics)
	blockProducer := NewBlockProducer(txPool, blockchain, gLog, stateMgr)
	sequencer := NewSequencer(ctx, blockProducer, gLog, metrics)

	err = sequencer.Start()
	if err != nil {
		b.Fatalf("Sequencer should start without error: %v", err)
	}

	err = sequencer.stateMgr.WaitForState(SequencerStateType, Started, 500*time.Millisecond)
	if err != nil {
		b.Fatalf("Sequencer should reach Started state: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tx := &types.Transaction{
			ID:        sha256.Sum256([]byte("tx" + strconv.Itoa(i+1))),
			Sender:    createAddress("did:peer:sender1"),
			Recipient: createAddress("did:peer:recipient1"),
			Amount:    1000,
			Fee:       10,
			Nonce:     uint64(1), // Resetting nonce to 1 to avoid mismatch issues
			Timestamp: time.Now().Unix(),
			Payload:   []byte("benchmark payload"),
		}

		tErr := txPool.AddTransaction(tx)
		if tErr != nil {
			b.Fatalf("Failed to add transaction to TxPool: %v", tErr)
		}

		time.Sleep(10 * time.Millisecond) // Reduced sleep to minimize artificial delay during benchmarking
	}

	err = sequencer.Stop()
	if err != nil {
		b.Fatalf("Sequencer should stop without error: %v", err)
	}

	err = sequencer.stateMgr.WaitForState(SequencerStateType, Stopped, 500*time.Millisecond)
	if err != nil {
		b.Fatalf("Sequencer should reach Stopped state: %v", err)
	}
}
