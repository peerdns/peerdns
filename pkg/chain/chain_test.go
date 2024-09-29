// pkg/chain/blockchain_test.go
package chain

import (
	"context"
	"os"
	"testing"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestBlockchain_AddAndGetBlock(t *testing.T) {
	// Initialize logger
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, err := loggerConfig.Build()
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Set up in-memory storage
	ctx := context.Background()
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "chain_test",
				Path:            "./data/test_chain.mdbx",
				MaxReaders:      126,
				MaxSize:         1,
				MinSize:         1,
				GrowthStep:      10 * 1024 * 1024,
				FilePermissions: 0600,
			},
		},
	}
	storageManager, err := storage.NewManager(ctx, mdbxConfig)
	if err != nil {
		t.Fatalf("Failed to create storage manager: %v", err)
	}
	defer func() {
		storageManager.Close()
		os.RemoveAll("./data/test_chain.mdbx")
	}()

	chainDb, err := storageManager.GetDb("chain_test")
	if err != nil {
		t.Fatalf("Failed to get chain database: %v", err)
	}

	blockchain := NewBlockchain(chainDb, logger)

	// Create a block
	block := &Block{
		Hash: []byte("block-hash"),
		Data: []byte("block-data"),
	}

	// Add block to blockchain
	err = blockchain.AddBlock(block)
	if err != nil {
		t.Fatalf("Failed to add block: %v", err)
	}

	// Retrieve block from blockchain
	retrievedBlock, err := blockchain.GetBlock(block.Hash)
	if err != nil {
		t.Fatalf("Failed to get block: %v", err)
	}

	if string(retrievedBlock.Data) != string(block.Data) {
		t.Errorf("Expected block data %s, got %s", block.Data, retrievedBlock.Data)
	}
}
