package chain

import "github.com/peerdns/peerdns/pkg/types"

// AddBlock adds a new block to the blockchain after validation.
func (bc *Blockchain) AddBlock(block *types.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate the block before adding
	if err := bc.ValidateBlock(block); err != nil {
		bc.logger.Error("Block validation failed", "error", err)
		return err
	}

	// Add the block to the ledger
	if err := bc.ledger.AddBlock(block); err != nil {
		bc.logger.Error("Failed to add block to ledger", "error", err)
		return err
	}

	// Update the head
	bc.head = block
	bc.logger.Info("Block added to the chain", "blockIndex", block.Index, "blockHash", block.Hash.Hex())

	return nil
}

// GetLatestBlock returns the latest block in the chain.
func (bc *Blockchain) GetLatestBlock() (*types.Block, error) {
	return bc.ledger.GetLatestBlock()
}

// GetBlockByIndex retrieves a block by its index.
func (bc *Blockchain) GetBlockByIndex(index uint64) (*types.Block, error) {
	return bc.ledger.GetBlockByIndex(index)
}

// GetBlockByHash retrieves a block by its hash.
func (bc *Blockchain) GetBlockByHash(hash types.Hash) (*types.Block, error) {
	return bc.ledger.GetBlock(hash)
}

func (bc *Blockchain) GetTransactionByHash(hash types.Hash) (*types.Transaction, error) {
	return bc.ledger.GetTransaction(hash)
}

func (bc *Blockchain) ListBlocks() ([]*types.Block, error) {
	return bc.ledger.ListBlocks()
}
