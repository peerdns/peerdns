package chain

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/genesis"
	"github.com/peerdns/peerdns/pkg/ledger"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	dbPath string
)

// Helper function to create a types.Address from a string by hashing and taking the first 20 bytes.
func createAddress(id string) types.Address {
	hash := sha256.Sum256([]byte(id))
	var addrBytes [types.AddressSize]byte
	copy(addrBytes[:], hash[:types.AddressSize])
	return addrBytes
}

// Helper function to set up the blockchain for testing.
func setupBlockchainTest(t *testing.T) (*Blockchain, string, config.Mdbx) {
	ctx := context.Background()

	gLog, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err)

	tempDir := t.TempDir()
	dbPath = filepath.Join(tempDir, "blockchain_test.db")
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

	blockchain, err := NewBlockchain(ctx, gLog, ldgr, genesisConfig)
	require.NoError(t, err, "Failed to initialize blockchain")

	// Set the head of the blockchain to the genesis block
	blockchain.head = genesisBlock

	return blockchain, dbPath, storageConfig
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

func TestBlockchain(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T, bc *Blockchain)
	}{
		{
			name: "TestAddBlock",
			testFunc: func(t *testing.T, bc *Blockchain) {
				// Get the current head to determine the correct block index
				currentHead, err := bc.GetLatestBlock()
				require.NoError(t, err, "Failed to get latest block")

				nextIndex := currentHead.Index + 1

				// Create and add a block to the blockchain
				tx := &types.Transaction{
					ID:        sha256.Sum256([]byte("tx1")),
					Sender:    createAddress("did:peer:sender1"),
					Recipient: createAddress("did:peer:recipient1"),
					Amount:    100,
					Fee:       1,
					Nonce:     1,
				}
				block, err := types.NewBlock(nextIndex, bc.head.Hash, []*types.Transaction{tx}, createAddress("did:peer:validator1"), nextIndex)
				require.NoError(t, err, "Failed to create new block")

				err = bc.AddBlock(block)
				assert.NoError(t, err, "Failed to add block to blockchain")
			},
		},
		{
			name: "TestGetLatestBlock",
			testFunc: func(t *testing.T, bc *Blockchain) {
				// Get the current head to determine the correct block index
				currentHead, err := bc.GetLatestBlock()
				require.NoError(t, err, "Failed to get latest block")

				nextIndex := currentHead.Index + 1

				// Create and add a block, then retrieve the latest block
				tx := &types.Transaction{
					ID:        sha256.Sum256([]byte("tx2")),
					Sender:    createAddress("did:peer:sender1"),
					Recipient: createAddress("did:peer:recipient1"),
					Amount:    200,
					Fee:       1,
					Nonce:     1,
				}
				block, err := types.NewBlock(nextIndex, bc.head.Hash, []*types.Transaction{tx}, createAddress("did:peer:validator1"), nextIndex)
				require.NoError(t, err, "Failed to create new block")

				err = bc.AddBlock(block)
				require.NoError(t, err, "Failed to add block to blockchain")

				latestBlock, err := bc.GetLatestBlock()
				require.NoError(t, err, "Failed to get latest block")
				assert.Equal(t, block.Hash, latestBlock.Hash, "Latest block hash does not match expected value")
			},
		},
		{
			name: "TestValidateChain",
			testFunc: func(t *testing.T, bc *Blockchain) {
				// Get the current head to determine the correct block index
				currentHead, err := bc.GetLatestBlock()
				require.NoError(t, err, "Failed to get latest block")

				nextIndex := currentHead.Index + 1

				// Create and add blocks, then validate the chain
				tx1 := &types.Transaction{
					ID:        sha256.Sum256([]byte("tx3")),
					Sender:    createAddress("did:peer:sender1"),
					Recipient: createAddress("did:peer:recipient1"),
					Amount:    300,
					Fee:       1,
					Nonce:     1,
				}
				block1, err := types.NewBlock(nextIndex, bc.head.Hash, []*types.Transaction{tx1}, createAddress("did:peer:validator1"), nextIndex)
				require.NoError(t, err, "Failed to create block1")
				err = bc.AddBlock(block1)
				require.NoError(t, err, "Failed to add block1 to blockchain")

				valid, err := bc.ValidateChain()
				assert.NoError(t, err, "Chain validation failed")
				assert.True(t, valid, "Chain should be valid")
			},
		},
		{
			name: "TestListBlocks",
			testFunc: func(t *testing.T, bc *Blockchain) {
				// Get the current head to determine the correct block index
				currentHead, err := bc.GetLatestBlock()
				require.NoError(t, err, "Failed to get latest block")

				nextIndex := currentHead.Index + 1

				// Create and add multiple blocks, then list them
				tx1 := &types.Transaction{
					ID:        sha256.Sum256([]byte("tx4")),
					Sender:    createAddress("did:peer:sender1"),
					Recipient: createAddress("did:peer:recipient1"),
					Amount:    400,
					Fee:       1,
					Nonce:     1,
				}
				block1, err := types.NewBlock(nextIndex, bc.head.Hash, []*types.Transaction{tx1}, createAddress("did:peer:validator1"), nextIndex)
				require.NoError(t, err, "Failed to create block1")
				err = bc.AddBlock(block1)
				require.NoError(t, err, "Failed to add block1 to blockchain")

				tx2 := &types.Transaction{
					ID:        sha256.Sum256([]byte("tx5")),
					Sender:    createAddress("did:peer:sender1"),
					Recipient: createAddress("did:peer:recipient2"),
					Amount:    500,
					Fee:       1,
					Nonce:     2,
				}
				nextIndex++
				block2, err := types.NewBlock(nextIndex, block1.Hash, []*types.Transaction{tx2}, createAddress("did:peer:validator1"), nextIndex)
				require.NoError(t, err, "Failed to create block2")
				err = bc.AddBlock(block2)
				require.NoError(t, err, "Failed to add block2 to blockchain")

				blocks, err := bc.ListBlocks()
				require.NoError(t, err, "Failed to list blocks")
				assert.Len(t, blocks, 3, "Should have three blocks (including genesis)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc, _, _ := setupBlockchainTest(t)
			defer bc.ledger.Close()

			tt.testFunc(t, bc)
		})
	}
}
