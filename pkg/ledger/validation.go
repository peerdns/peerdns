// pkg/ledger/validation.go

package ledger

import (
	"fmt"
	"time"

	"github.com/peerdns/peerdns/pkg/types"
)

// ValidateBlock performs comprehensive validation on the given block.
func (l *Ledger) ValidateBlock(block *types.Block) error {
	// 1. Verify that the block's index is sequential.
	if block.Index == 0 && block.PreviousHash != (types.Hash{}) {
		return fmt.Errorf("genesis block must have a zero previous hash")
	}
	if block.Index > 0 {
		prevBlock, err := l.GetBlockByHash(block.PreviousHash)
		if err != nil {
			return fmt.Errorf("previous block not found: %w", err)
		}
		if block.Index != prevBlock.Index+1 {
			return fmt.Errorf("block index %d is not sequential", block.Index)
		}
	}

	// 2. Verify that the block's hash is correctly computed.
	expectedHash := block.ComputeHash()
	if !types.HashEqual(block.Hash, expectedHash) {
		return fmt.Errorf("block hash is invalid")
	}

	// 3. Verify that the block's Merkle root is correctly computed.
	expectedMerkleRoot := block.ComputeMerkleRoot()
	if !types.HashEqual(block.MerkleRoot, expectedMerkleRoot) {
		return fmt.Errorf("block Merkle root is invalid")
	}

	// 4. Verify that the block's timestamp is reasonable.
	currentTime := time.Now().Unix()
	if block.Timestamp > currentTime+MaxTimeDrift {
		return fmt.Errorf("block timestamp %d is too far in the future", block.Timestamp)
	}
	if block.Index > 0 {
		prevBlock, err := l.GetBlockByHash(block.PreviousHash)
		if err != nil {
			return fmt.Errorf("previous block not found: %w", err)
		}
		if block.Timestamp < prevBlock.Timestamp { // In production it should be <=
			return fmt.Errorf("block timestamp %d is not greater than previous block timestamp %d", block.Timestamp, prevBlock.Timestamp)
		}
	}

	// 5. Verify the block's signature.
	/*	if !signatures.VerifyBlockSignature(block) {
		return fmt.Errorf("block signature is invalid")
	}*/

	// Determine if this is the genesis block
	isGenesisBlock := block.Index == 0

	// 6. Verify each transaction in the block.
	for _, tx := range block.Transactions {
		if err := l.validateTransaction(tx, isGenesisBlock); err != nil {
			return fmt.Errorf("invalid transaction %s: %w", tx.ID.Hex(), err)
		}
	}

	// 7. Consensus-specific validation (e.g., Proof-of-Work).
	if err := l.validateConsensusRules(block); err != nil {
		return fmt.Errorf("block does not meet consensus rules: %w", err)
	}

	return nil
}

// validateTransaction performs validation on a single transaction.
// It accepts a flag indicating if it's validating a genesis block transaction.
func (l *Ledger) validateTransaction(tx *types.Transaction, isGenesisBlock bool) error {
	// 1. Verify the transaction signature.
	/*	if !signatures.VerifyTransactionSignature(tx) {
		return fmt.Errorf("transaction signature is invalid")
	}*/

	// 2. Check that the sender's address is valid.
	if tx.Sender.IsZero() {
		return fmt.Errorf("transaction sender address is zero")
	}

	// 3. Check that the transaction amount and fee are positive.
	if tx.Amount <= 0 {
		return fmt.Errorf("transaction amount must be positive")
	}
	if tx.Fee < 0 {
		return fmt.Errorf("transaction fee cannot be negative")
	}

	if !isGenesisBlock {
		// 5. Check that the sender has sufficient balance.
		senderBalance := l.state.GetBalance(tx.Sender)
		if senderBalance < tx.Amount+tx.Fee {
			return fmt.Errorf("sender has insufficient balance")
		}

		// 6. Prevent double-spending by checking the transaction nonce.
		expectedNonce := l.state.GetNonce(tx.Sender) + 1
		if tx.Nonce != expectedNonce {
			return fmt.Errorf("invalid transaction nonce: expected %d, got %d", expectedNonce, tx.Nonce)
		}
	}

	// Additional checks can be added here.

	return nil
}

// validateConsensusRules performs consensus-specific validation on the block.
func (l *Ledger) validateConsensusRules(block *types.Block) error {
	// Example for Proof-of-Work:

	// 1. Check that the block's hash meets the required difficulty.
	target := CalculateTarget(block.Difficulty)
	if !blockHashMeetsTarget(block.Hash, target) {
		return fmt.Errorf("block hash does not meet difficulty target")
	}

	// Additional consensus-specific checks can be added here.

	return nil
}
