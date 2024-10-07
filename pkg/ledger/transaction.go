// pkg/ledger/transaction.go

package ledger

import (
	"encoding/hex"
	"fmt"

	"github.com/peerdns/peerdns/pkg/types"
)

// GetTransaction retrieves a transaction by its ID.
func (l *Ledger) GetTransaction(txID string) (*types.Transaction, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Iterate through blocks to find the transaction.
	blocks, err := l.ListBlocks()
	if err != nil {
		return nil, fmt.Errorf("failed to list blocks: %w", err)
	}

	for _, block := range blocks {
		for _, tx := range block.Transactions {
			if hex.EncodeToString(tx.ID[:]) == txID {
				return tx, nil
			}
		}
	}

	return nil, fmt.Errorf("transaction %s not found", txID)
}

// IterateTransactions applies a given function to all transactions.
func (l *Ledger) IterateTransactions(fn func(tx *types.Transaction) error) error {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	blocks, err := l.ListBlocks()
	if err != nil {
		return fmt.Errorf("failed to list blocks: %w", err)
	}

	for _, block := range blocks {
		for _, tx := range block.Transactions {
			if err := fn(tx); err != nil {
				return err
			}
		}
	}

	return nil
}

// SeekTransactions finds transactions matching a predicate.
func (l *Ledger) SeekTransactions(predicate func(tx *types.Transaction) bool) ([]*types.Transaction, error) {
	var matchedTxs []*types.Transaction

	err := l.IterateTransactions(func(tx *types.Transaction) error {
		if predicate(tx) {
			matchedTxs = append(matchedTxs, tx)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return matchedTxs, nil
}
