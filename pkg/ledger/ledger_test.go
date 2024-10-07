// pkg/ledger/ledger_test.go

package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a types.Address from a string by hashing and taking the first 20 bytes.
func createAddress(id string) types.Address {
	// Hash the input string to get a consistent length output
	hash := sha256.Sum256([]byte(id))
	// Take the first 20 bytes of the hash to create the address
	var addrBytes [types.AddressSize]byte
	copy(addrBytes[:], hash[:types.AddressSize])
	return addrBytes
}

// Helper function to create a test transaction with a given ID.
func createTestTransaction(id string) *types.Transaction {
	txID := sha256.Sum256([]byte(id))
	senderAddress := createAddress("did:peer:sender1")
	recipientAddress := createAddress("did:peer:recipient1")
	tx := &types.Transaction{
		ID:            txID,
		Sender:        senderAddress,
		Recipient:     recipientAddress,
		Amount:        1000,
		Fee:           10,
		Nonce:         1,
		Timestamp:     time.Now().Unix(),
		Payload:       []byte("test payload"),
		Signature:     []byte{},                 // Initialize Signature
		SignatureType: signatures.BlsSignerType, // Initialize SignatureType
	}
	return tx
}

func setupLedger(t *testing.T) (*Ledger, *accounts.Store) {
	ctx := context.Background()

	// Setup temporary directory for database storage
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ledger_test.db")

	// Storage configuration
	storageConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "ledger_test",
				Path:            dbPath,
				MaxReaders:      4096,
				MaxSize:         1,    // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
		},
	}

	// Initialize storage manager
	storageManager, err := storage.NewManager(ctx, storageConfig)
	require.NoError(t, err, "Failed to create storage manager")

	// Get the database
	db, err := storageManager.GetDb("ledger_test")
	require.NoError(t, err, "Failed to get database")

	// Initialize ledger
	ledger := NewLedger(ctx, db)

	// Initialize accounts store
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
				tx := createTestTransaction("tx1")

				validatorAddress := createAddress("did:peer:validator1")
				block, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create new block")

				err = l.AddBlock(block)
				assert.NoError(t, err, "Failed to add block")
			},
		},
		{
			name: "TestGetBlock",
			testFunc: func(t *testing.T, l *Ledger) {
				tx := createTestTransaction("tx2")

				validatorAddress := createAddress("did:peer:validator1")

				block, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create new block")

				err = l.AddBlock(block)
				require.NoError(t, err, "Failed to add block")

				blockHash := hex.EncodeToString(block.Hash[:])

				retrievedBlock, err := l.GetBlock(blockHash)
				require.NoError(t, err, "Failed to get block")
				assert.Equal(t, block.Index, retrievedBlock.Index)
				assert.Equal(t, block.Hash, retrievedBlock.Hash)
			},
		},
		{
			name: "TestValidateChain",
			testFunc: func(t *testing.T, l *Ledger) {
				// Create and add the genesis block with at least one allocation
				genesisConfig := &Genesis{
					Difficulty: 0,
					Timestamp:  time.Now().Unix(),
					Alloc: map[string]AllocItem{
						createAddress("did:peer:recipient1").Hex(): {Balance: "1000"},
					},
				}
				genesisBlock, err := CreateGenesisBlock(genesisConfig)
				require.NoError(t, err, "Failed to create genesis block")

				err = l.AddBlock(genesisBlock)
				require.NoError(t, err, "Failed to add genesis block")

				// Create and add a second block
				tx := createTestTransaction("tx3")
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
				// Create and add blocks
				tx1 := createTestTransaction("tx4")
				validatorAddress := createAddress("did:peer:validator1")
				block1, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx1}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block1")
				err = l.AddBlock(block1)
				require.NoError(t, err, "Failed to add block1")

				tx2 := createTestTransaction("tx5")
				block2, err := types.NewBlock(2, block1.Hash, []*types.Transaction{tx2}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block2")
				err = l.AddBlock(block2)
				require.NoError(t, err, "Failed to add block2")

				blocks, err := l.ListBlocks()
				require.NoError(t, err, "Failed to list blocks")
				assert.Len(t, blocks, 2, "Should have two blocks")
			},
		},
		{
			name: "TestTraverseSuccessors",
			testFunc: func(t *testing.T, l *Ledger) {
				// Create and add blocks
				tx1 := createTestTransaction("tx6")
				validatorAddress := createAddress("did:peer:validator1")
				block1, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx1}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block1")
				err = l.AddBlock(block1)
				require.NoError(t, err, "Failed to add block1")

				tx2 := createTestTransaction("tx7")
				block2, err := types.NewBlock(2, block1.Hash, []*types.Transaction{tx2}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block2")
				err = l.AddBlock(block2)
				require.NoError(t, err, "Failed to add block2")

				blockHash := hex.EncodeToString(block1.Hash[:])
				successors, err := l.TraverseSuccessors(blockHash)
				require.NoError(t, err, "Failed to traverse successors")
				assert.Len(t, successors, 1, "Should have one successor")
			},
		},
		{
			name: "TestGetTransaction",
			testFunc: func(t *testing.T, l *Ledger) {
				// Create and add a block with a transaction
				tx := createTestTransaction("tx8")
				validatorAddress := createAddress("did:peer:validator1")
				block, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block")
				err = l.AddBlock(block)
				require.NoError(t, err, "Failed to add block")

				txID := hex.EncodeToString(tx.ID[:])

				// Retrieve the transaction
				retrievedTx, err := l.GetTransaction(txID)
				require.NoError(t, err, "Failed to get transaction")
				assert.Equal(t, tx.ID, retrievedTx.ID, "Transaction IDs should match")
			},
		},
		{
			name: "TestIterateTransactions",
			testFunc: func(t *testing.T, l *Ledger) {
				// Create and add blocks with transactions
				tx1 := createTestTransaction("tx9")
				validatorAddress := createAddress("did:peer:validator1")
				block1, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx1}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block1")
				err = l.AddBlock(block1)
				require.NoError(t, err, "Failed to add block1")

				tx2 := createTestTransaction("tx10")
				block2, err := types.NewBlock(2, block1.Hash, []*types.Transaction{tx2}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block2")
				err = l.AddBlock(block2)
				require.NoError(t, err, "Failed to add block2")

				var txs []*types.Transaction
				err = l.IterateTransactions(func(tx *types.Transaction) error {
					txs = append(txs, tx)
					return nil
				})
				require.NoError(t, err, "Failed to iterate transactions")
				assert.Len(t, txs, 2, "Should have two transactions")
			},
		},
		{
			name: "TestSeekTransactions",
			testFunc: func(t *testing.T, l *Ledger) {
				// Create and add blocks with transactions
				tx1 := createTestTransaction("tx11")
				tx1.Sender = createAddress("did:peer:sender1")

				validatorAddress := createAddress("did:peer:validator1")
				block1, err := types.NewBlock(1, [types.HashSize]byte{}, []*types.Transaction{tx1}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block1")
				err = l.AddBlock(block1)
				require.NoError(t, err, "Failed to add block1")

				tx2 := createTestTransaction("tx12")
				tx2.Sender = createAddress("did:peer:sender2")

				block2, err := types.NewBlock(2, block1.Hash, []*types.Transaction{tx2}, validatorAddress, 1)
				require.NoError(t, err, "Failed to create block2")
				err = l.AddBlock(block2)
				require.NoError(t, err, "Failed to add block2")

				// Seek transactions from sender1
				targetSender := createAddress("did:peer:sender1")
				matchedTxs, err := l.SeekTransactions(func(tx *types.Transaction) bool {
					return tx.Sender == targetSender
				})
				require.NoError(t, err, "Failed to seek transactions")
				assert.Len(t, matchedTxs, 1, "Should have one matching transaction")
			},
		},
		{
			name: "TestGenesisBlockLoading",
			testFunc: func(t *testing.T, l *Ledger) {
				// Assuming you have a genesis.yaml in the testdata directory
				genesisPath := filepath.Join("../../testdata", "genesis.yaml")
				genesisConfig, err := LoadGenesis(genesisPath)
				require.NoError(t, err, "Failed to load genesis config")

				genesisBlock, err := CreateGenesisBlock(genesisConfig)
				require.NoError(t, err, "Failed to create genesis block")

				err = l.AddBlock(genesisBlock)
				require.NoError(t, err, "Failed to add genesis block")

				// Verify that the genesis block is correctly added
				blockHash := hex.EncodeToString(genesisBlock.Hash[:])
				retrievedBlock, err := l.GetBlock(blockHash)
				require.NoError(t, err, "Failed to get genesis block")
				assert.Equal(t, genesisBlock.Index, retrievedBlock.Index)
				assert.Equal(t, genesisBlock.Hash, retrievedBlock.Hash)
			},
		},
		{
			name: "TestBlockSerializationDeserialization",
			testFunc: func(t *testing.T, l *Ledger) {
				// Create a block with multiple transactions
				txs := []*types.Transaction{}
				for i := 0; i < 10; i++ {
					txID := fmt.Sprintf("tx%d", i)
					tx := createTestTransaction(txID)
					txs = append(txs, tx)
				}

				validatorAddress := createAddress("did:peer:validator_benchmark")
				block, err := types.NewBlock(1, [types.HashSize]byte{}, txs, validatorAddress, 1)
				require.NoError(t, err, "Failed to create new block")

				// Serialize the block
				serializedBlock, err := block.Serialize()
				require.NoError(t, err, "Failed to serialize block")

				// Simulate storing and retrieving the block from storage
				blockHash := hex.EncodeToString(block.Hash[:])
				key := []byte(blockHash)
				err = l.storage.Set(key, serializedBlock)
				require.NoError(t, err, "Failed to store block in database")

				// Retrieve and deserialize the block
				retrievedData, err := l.storage.Get(key)
				require.NoError(t, err, "Failed to get block from database")

				deserializedBlock, err := types.DeserializeBlock(retrievedData)
				require.NoError(t, err, "Failed to deserialize block")

				// Compare the original and deserialized blocks
				assert.Equal(t, block.Index, deserializedBlock.Index)
				assert.Equal(t, block.Hash, deserializedBlock.Hash)
				assert.Equal(t, block.ValidatorID, deserializedBlock.ValidatorID)
				assert.Equal(t, len(block.Transactions), len(deserializedBlock.Transactions))
				for i := range block.Transactions {
					assert.Equal(t, block.Transactions[i].ID, deserializedBlock.Transactions[i].ID)
					assert.Equal(t, block.Transactions[i].Sender, deserializedBlock.Transactions[i].Sender)
					assert.Equal(t, block.Transactions[i].Recipient, deserializedBlock.Transactions[i].Recipient)
					assert.Equal(t, block.Transactions[i].Amount, deserializedBlock.Transactions[i].Amount)
					assert.Equal(t, block.Transactions[i].Fee, deserializedBlock.Transactions[i].Fee)
					assert.Equal(t, block.Transactions[i].Nonce, deserializedBlock.Transactions[i].Nonce)
					assert.Equal(t, block.Transactions[i].Timestamp, deserializedBlock.Transactions[i].Timestamp)
					assert.Equal(t, block.Transactions[i].Payload, deserializedBlock.Transactions[i].Payload)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, _ := setupLedger(t)
			defer l.Close()

			tt.testFunc(t, l)
		})
	}
}
