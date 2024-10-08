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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"time"
)

// Helper function to create a types.Address from a string by hashing and taking the first 20 bytes.
func createAddress(id string) types.Address {
	hash := sha256.Sum256([]byte(id))
	var addrBytes [types.AddressSize]byte
	copy(addrBytes[:], hash[:types.AddressSize])
	return addrBytes
}

// Helper function to set up the blockchain for testing.
func setupBlockchainTest(t *testing.T) (*chain.Blockchain, *ledger.Ledger, *types.Block, config.Mdbx) {
	ctx := context.Background()

	gLog, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err)

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "blockchain_test.db")
	storageConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "blockchain_test",
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
	require.NoError(t, err, "Failed to create storage manager")
	db, err := storageManager.GetDb("blockchain_test")
	require.NoError(t, err, "Failed to get database")

	ldgr, err := ledger.NewLedger(ctx, db)
	require.NoError(t, err, "Failed to create new ledger")
	genesisConfig := &genesis.Genesis{
		Difficulty: 0,
		Timestamp:  time.Now().Unix(),
		Alloc: map[string]genesis.AllocItem{
			createAddress("did:peer:sender1").Hex(): {Balance: "1000000"},
		},
	}

	// Create and add the genesis block
	genesisBlock := addGenesisBlock(t, ldgr)

	blockchain, err := chain.NewBlockchain(ctx, gLog, ldgr, genesisConfig)
	require.NoError(t, err, "Failed to initialize blockchain")

	return blockchain, ldgr, genesisBlock, storageConfig
}

// Helper function to add the genesis block to the ledger.
func addGenesisBlock(t *testing.T, l *ledger.Ledger) *types.Block {
	genesisValidator := createAddress("did:peer:genesis-validator")
	genesisConfig := &genesis.Genesis{
		Difficulty: 0,
		Timestamp:  time.Now().Unix(),
		Alloc: map[string]genesis.AllocItem{
			createAddress("did:peer:sender1").Hex():    {Balance: "10000000"},
			createAddress("did:peer:recipient1").Hex(): {Balance: "10000000"},
			createAddress("did:peer:validator1").Hex(): {Balance: "10000000"},
			createAddress("did:peer:sender2").Hex():    {Balance: "10000000"},
		},
	}
	genesisBlock, err := genesis.CreateGenesisBlock(genesisValidator, genesisConfig)
	require.NoError(t, err, "Failed to create genesis block")

	err = l.AddBlock(genesisBlock)
	require.NoError(t, err, "Failed to add genesis block to ledger")

	return genesisBlock
}

func TestSequencer_StartAndStop(t *testing.T) {
	gLog, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err)

	txPool := txpool.NewTxPool()                  // Use a real TxPool instance
	blockchain, _, _, _ := setupBlockchainTest(t) // Set up real ledger instance

	metrics := &SequencerMetrics{}
	stateMgr := NewStateManager(gLog, metrics)
	blockProducer := NewBlockProducer(txPool, blockchain, gLog, stateMgr)

	ctx := context.Background()
	sequencer := NewSequencer(ctx, blockProducer, gLog, metrics)

	tests := []struct {
		name           string
		startSequencer bool
		expectError    bool
	}{
		{"Start and Stop Sequencer", true, false},
		{"Stop without Start", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.startSequencer {
				err := sequencer.Start()
				assert.NoError(t, err, "Sequencer should start without error")
				err = sequencer.stateMgr.WaitForState(SequencerStateType, Started, 100*time.Millisecond)
				assert.NoError(t, err, "Sequencer should reach Started state")
			}

			err := sequencer.Stop()
			assert.NoError(t, err, "Sequencer should stop without error")
			err = sequencer.stateMgr.WaitForState(SequencerStateType, Stopped, 100*time.Millisecond)
			assert.NoError(t, err, "Sequencer should reach Stopped state")
		})
	}
}

func TestSequencer_ProduceBlock(t *testing.T) {
	gLog, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err)

	txPool := txpool.NewTxPool()                             // Use a real TxPool instance
	blockchain, _, genesisBlock, _ := setupBlockchainTest(t) // Set up real ledger instance

	metrics := &SequencerMetrics{}
	stateMgr := NewStateManager(gLog, metrics)
	blockProducer := NewBlockProducer(txPool, blockchain, gLog, stateMgr)

	ctx := context.Background()
	sequencer := NewSequencer(ctx, blockProducer, gLog, metrics)

	err = sequencer.Start()
	assert.NoError(t, err, "Sequencer should start without error")

	err = sequencer.stateMgr.WaitForState(SequencerStateType, Started, 500*time.Millisecond)
	assert.NoError(t, err, "Sequencer should reach Started state")

	// Create a transaction and add it to the TxPool
	tx := &types.Transaction{
		ID:        sha256.Sum256([]byte("tx1")),
		Sender:    createAddress("did:peer:sender1"),
		Recipient: createAddress("did:peer:recipient1"),
		Amount:    1000,
		Fee:       10,
		Nonce:     1,
		Timestamp: time.Now().Unix(),
		Payload:   []byte("test payload"),
	}

	tErr := txPool.AddTransaction(tx)
	require.NoError(t, tErr)

	// Increase wait time to handle potential timing issues
	time.Sleep(1000 * time.Millisecond)

	err = sequencer.Stop()
	assert.NoError(t, err, "Sequencer should stop without error")

	err = sequencer.stateMgr.WaitForState(SequencerStateType, Stopped, 500*time.Millisecond)
	assert.NoError(t, err, "Sequencer should reach Stopped state")

	// Verify that a new block was forged and added to the ledger
	latestBlock, err := blockchain.GetLatestBlock()
	assert.NoError(t, err, "Failed to get latest block")
	assert.Equal(t, genesisBlock.Index+1, latestBlock.Index, "New block index should be incremented by 1 from genesis")
	assert.Equal(t, 1, len(latestBlock.Transactions), "New block should contain 1 transaction")
	assert.Equal(t, tx.ID, latestBlock.Transactions[0].ID, "Transaction in the block should match the one added to TxPool")
}
