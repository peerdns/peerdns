package chain

import (
	"fmt"
	"github.com/peerdns/peerdns/pkg/types"
)

func (bc *Blockchain) HandleIncomingBlock(block *types.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate the incoming block
	if err := bc.ValidateBlock(block); err != nil {
		bc.logger.Error("Incoming block validation failed", "error", err)
		return err
	}

	// Check if the incoming block extends the current chain
	if block.PreviousHash.Equal(bc.head.Hash) {
		// Extend the chain
		if err := bc.ledger.AddBlock(block); err != nil {
			bc.logger.Error("Failed to add incoming block to ledger", "error", err)
			return err
		}
		bc.head = block
		bc.logger.Info("Incoming block added to the chain", "blockIndex", block.Index, "blockHash", block.Hash.Hex())
		return nil
	}

	// Handle forks: Check if the incoming block could be part of a longer chain
	// This requires fetching the chain from the incoming block back to a common ancestor
	// and determining if it should replace the current chain

	// Placeholder for fork handling logic

	return fmt.Errorf("block does not extend the current chain and fork handling not implemented")
}
