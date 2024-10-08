package ledger

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/stretchr/testify/require"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"
)

// Helper function to create a benchmark transaction with a given ID.
func createBenchmarkTransaction(id string) *types.Transaction {
	txID := sha256.Sum256([]byte(id))
	senderAddress := createAddress("did:peer:sender_benchmark")
	recipientAddress := createAddress("did:peer:recipient_benchmark")
	tx := &types.Transaction{
		ID:        txID,
		Sender:    senderAddress,
		Recipient: recipientAddress,
		Amount:    big.NewInt(1000).Uint64(), // Assuming Amount is uint64
		Fee:       10,
		Nonce:     1,
		Timestamp: time.Now().Unix(),
		Payload:   []byte("benchmark payload"),
	}
	return tx
}

func setupBenchmarkLedger(b *testing.B) (*Ledger, error) {
	ctx := context.Background()

	// Setup temporary directory for database storage
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "ledger_benchmark.db")

	// Storage configuration
	storageConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "ledger_benchmark",
				Path:            dbPath,
				MaxReaders:      4096,
				MaxSize:         10,       // in GB for testing purposes
				MinSize:         6 * 1024, // in MB
				GrowthStep:      16 * 1024,
				FilePermissions: 0600,
			},
		},
	}

	// Initialize storage manager
	storageManager, err := storage.NewManager(ctx, storageConfig)
	if err != nil {
		b.Fatalf("Failed to create storage manager: %v", err)
	}

	// Get the database
	db, err := storageManager.GetDb("ledger_benchmark")
	if err != nil {
		b.Fatalf("Failed to get database: %v", err)
	}

	return NewLedger(ctx, db)
}

func BenchmarkLedger(b *testing.B) {
	b.Run("BenchmarkAddBlock", func(b *testing.B) {
		l, lErr := setupBenchmarkLedger(b)
		require.NoError(b, lErr)
		defer l.Close()

		minerAddress := createAddress("did:peer:miner_benchmark")

		for i := 0; i < b.N; i++ {
			tx := createBenchmarkTransaction(fmt.Sprintf("tx_benchmark_%d", i))
			block, err := types.NewBlock(uint64(i), [types.HashSize]byte{}, []*types.Transaction{tx}, minerAddress, 1)
			if err != nil {
				b.Fatalf("Failed to create new block: %v", err)
			}

			err = l.AddBlock(block)
			if err != nil {
				b.Fatalf("Failed to add block: %v", err)
			}
		}
	})

	b.Run("BenchmarkGetBlock", func(b *testing.B) {
		l, lErr := setupBenchmarkLedger(b)
		require.NoError(b, lErr)
		defer l.Close()

		// Add a block to retrieve
		tx := createBenchmarkTransaction("tx_benchmark_lookup")
		minerAddress := createAddress("did:peer:miner_benchmark")
		block, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx}, minerAddress, 1)
		if err != nil {
			b.Fatalf("Failed to create new block: %v", err)
		}

		err = l.AddBlock(block)
		if err != nil {
			b.Fatalf("Failed to add block: %v", err)
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := l.GetBlock(block.Hash)
			if err != nil {
				b.Fatalf("Failed to get block: %v", err)
			}
		}
	})

	b.Run("BenchmarkGetBlockFromStorage", func(b *testing.B) {
		l, lErr := setupBenchmarkLedger(b)
		require.NoError(b, lErr)
		defer l.Close()

		// Add a block to retrieve
		tx := createBenchmarkTransaction("tx_benchmark_lookup")
		minerAddress := createAddress("did:peer:miner_benchmark")
		block, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx}, minerAddress, 1)
		if err != nil {
			b.Fatalf("Failed to create new block: %v", err)
		}

		err = l.AddBlock(block)
		if err != nil {
			b.Fatalf("Failed to add block: %v", err)
		}

		// Remove the block from the in-memory graph to force a cache miss
		l.graph.RemoveVertex(block.Hash)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := l.GetBlock(block.Hash)
			if err != nil {
				b.Fatalf("Failed to get block: %v", err)
			}

			// Remove the block from the graph again to ensure it's not cached
			l.graph.RemoveVertex(block.Hash)
		}
	})
}
