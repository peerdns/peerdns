package ledger

import (
	"context"
	"crypto/sha256"
	"github.com/peerdns/peerdns/pkg/genesis"
	"path/filepath"
	"testing"
	"time"

	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a types.Address from a string by hashing and taking the first 20 bytes.
func createAddress(id string) types.Address {
	hash := sha256.Sum256([]byte(id))
	var addrBytes [types.AddressSize]byte
	copy(addrBytes[:], hash[:types.AddressSize])
	return addrBytes
}

// Helper function to create a test transaction with a given ID.
func createTestTransaction(id string, nonce uint64) *types.Transaction {
	txID := sha256.Sum256([]byte(id))
	senderAddress := createAddress("did:peer:sender1")
	recipientAddress := createAddress("did:peer:recipient1")
	return &types.Transaction{
		ID:            txID,
		Sender:        senderAddress,
		Recipient:     recipientAddress,
		Amount:        1000,
		Fee:           10,
		Nonce:         nonce,
		Timestamp:     time.Now().Unix(),
		Payload:       []byte("test payload"),
		Signature:     []byte{},
		SignatureType: signatures.BlsSignerType,
	}
}

// Helper function to add the genesis block to the ledger.
func addGenesisBlock(t *testing.T, l *Ledger) *types.Block {
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
	require.NoError(t, err, "Failed to add genesis block")

	return genesisBlock
}

func setupLedger(t *testing.T) (*Ledger, *accounts.Store) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ledger_test.db")

	storageConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "ledger_test",
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

	db, err := storageManager.GetDb("ledger_test")
	require.NoError(t, err, "Failed to get database")

	ledger := NewLedger(ctx, db)

	cfg := config.Identity{
		Enabled:  true,
		BasePath: tempDir,
	}

	accountStore, err := accounts.NewStore(cfg, nil)
	require.NoError(t, err, "Failed to create accounts store")

	return ledger, accountStore
}

func TestLedger(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T, l *Ledger)
	}{
		{
			name: "TestAddBlock",
			testFunc: func(t *testing.T, l *Ledger) {
				genesisBlock := addGenesisBlock(t, l)

				tx := createTestTransaction("tx1", 1)
				validatorAddress := createAddress("did:peer:validator1")
				block, err := types.NewBlock(1, genesisBlock.Hash, []*types.Transaction{tx}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create new block")

				err = l.AddBlock(block)
				assert.NoError(t, err, "Failed to add block")
			},
		},
		{
			name: "TestGetBlock",
			testFunc: func(t *testing.T, l *Ledger) {
				genesisBlock := addGenesisBlock(t, l)

				tx := createTestTransaction("tx2", 1)
				validatorAddress := createAddress("did:peer:validator1")
				block, err := types.NewBlock(1, genesisBlock.Hash, []*types.Transaction{tx}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create new block")

				err = l.AddBlock(block)
				require.NoError(t, err, "Failed to add block")

				retrievedBlock, err := l.GetBlock(block.Hash)
				require.NoError(t, err, "Failed to get block")
				assert.Equal(t, block.Index, retrievedBlock.Index)
				assert.Equal(t, block.Hash, retrievedBlock.Hash)
			},
		},
		{
			name: "TestValidateChain",
			testFunc: func(t *testing.T, l *Ledger) {
				genesisBlock := addGenesisBlock(t, l)

				tx := createTestTransaction("tx3", 1)
				validatorAddress := createAddress("did:peer:validator1")
				block, err := types.NewBlock(1, genesisBlock.Hash, []*types.Transaction{tx}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create new block")
				err = l.AddBlock(block)
				require.NoError(t, err, "Failed to add block")

				valid, err := l.ValidateChain()
				assert.NoError(t, err, "Chain validation failed")
				assert.True(t, valid, "Chain should be valid")
			},
		},
		{
			name: "TestListBlocks",
			testFunc: func(t *testing.T, l *Ledger) {
				genesisBlock := addGenesisBlock(t, l)

				tx1 := createTestTransaction("tx4", 1)
				validatorAddress := createAddress("did:peer:validator1")
				block1, err := types.NewBlock(1, genesisBlock.Hash, []*types.Transaction{tx1}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block1")
				err = l.AddBlock(block1)
				require.NoError(t, err, "Failed to add block1")

				tx2 := createTestTransaction("tx5", 2)
				block2, err := types.NewBlock(2, block1.Hash, []*types.Transaction{tx2}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block2")
				err = l.AddBlock(block2)
				require.NoError(t, err, "Failed to add block2")

				blocks, err := l.ListBlocks()
				require.NoError(t, err, "Failed to list blocks")
				assert.Len(t, blocks, 3, "Should have three blocks (including genesis)")
			},
		},
		// ... Additional tests omitted for brevity ...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, _ := setupLedger(t)
			defer l.Close()

			tt.testFunc(t, l)
		})
	}
}

func TestLedger_Persistence(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ledger_test.db")

	storageConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "ledger_test",
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

	db, err := storageManager.GetDb("ledger_test")
	require.NoError(t, err, "Failed to get database")

	ledger := NewLedger(ctx, db)

	genesisBlock := addGenesisBlock(t, ledger)

	err = ledger.Close()
	require.NoError(t, err, "Failed to close ledger")

	storageManager, err = storage.NewManager(ctx, storageConfig)
	require.NoError(t, err, "Failed to recreate storage manager after closing")

	db, err = storageManager.GetDb("ledger_test")
	require.NoError(t, err, "Failed to get database after reinitializing storage manager")

	ledger = NewLedger(ctx, db)
	defer ledger.Close()

	retrievedBlock, err := ledger.GetBlock(genesisBlock.Hash)
	require.NoError(t, err, "Failed to retrieve genesis block after reinitialization")
	assert.Equal(t, genesisBlock.Hash, retrievedBlock.Hash, "Block hashes should match")
}
