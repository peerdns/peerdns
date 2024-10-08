// pkg/txpool/txpool.go

package txpool

import (
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/peerdns/peerdns/pkg/types"
)

// Constants for TxPool
const (
	DefaultMaxPoolSize     = 1000             // Maximum number of transactions in the pool
	DefaultTxExpiry        = 10 * time.Minute // Duration after which a transaction expires
	DefaultCleanupInterval = 1 * time.Minute  // Interval between cleanup operations
)

// TxPool manages a pool of pending transactions.
type TxPool struct {
	mu                  sync.RWMutex
	transactions        map[types.Hash]*types.Transaction
	maxSize             int
	txExpiry            time.Duration
	validateTransaction func(tx *types.Transaction) error
	cleanupInterval     time.Duration
}

// NewTxPool initializes and returns a new TxPool instance with default settings.
func NewTxPool() *TxPool {
	pool := &TxPool{
		transactions:    make(map[types.Hash]*types.Transaction),
		maxSize:         DefaultMaxPoolSize,
		txExpiry:        DefaultTxExpiry,
		cleanupInterval: DefaultCleanupInterval,
	}
	pool.validateTransaction = pool.validateTransactionDefault
	go pool.cleanupExpiredTransactions()
	return pool
}

// NewTxPoolWithOptions initializes and returns a new TxPool instance with custom settings.
func NewTxPoolWithOptions(maxSize int, txExpiry, cleanupInterval time.Duration, validateFn func(tx *types.Transaction) error) *TxPool {
	pool := &TxPool{
		transactions:        make(map[types.Hash]*types.Transaction),
		maxSize:             maxSize,
		txExpiry:            txExpiry,
		validateTransaction: validateFn,
		cleanupInterval:     cleanupInterval,
	}
	if pool.validateTransaction == nil {
		pool.validateTransaction = pool.validateTransactionDefault
	}
	go pool.cleanupExpiredTransactionsWithInterval(cleanupInterval)
	return pool
}

// validateTransactionDefault is the default validation function.
func (tp *TxPool) validateTransactionDefault(tx *types.Transaction) error {
	// Placeholder for actual validation logic
	// Example validations:
	// - Verify the transaction signature
	// - Check nonce for the sender
	// - Ensure sender has sufficient balance
	// - Prevent double-spending

	// For demonstration, we'll implement basic validation:
	if tx.Amount == 0 {
		return errors.New("transaction amount must be greater than zero")
	}
	// tx.Fee is uint64, cannot be negative
	// Additional validations can be added here
	return nil
}

// SetMaxSize sets the maximum number of transactions the pool can hold.
func (tp *TxPool) SetMaxSize(max int) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.maxSize = max
}

// SetTxExpiry sets the duration after which transactions expire.
func (tp *TxPool) SetTxExpiry(expiry time.Duration) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.txExpiry = expiry
}

// AddTransaction adds a new transaction to the pool after validating it.
// Returns an error if the transaction is invalid or the pool is full.
func (tp *TxPool) AddTransaction(tx *types.Transaction) error {
	// Validate the transaction
	if err := tp.validateTransaction(tx); err != nil {
		return err
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()

	txID := tx.ID
	if _, exists := tp.transactions[txID]; exists {
		return errors.New("transaction already exists in the pool")
	}

	// Check if pool is full
	if len(tp.transactions) >= tp.maxSize {
		// Implement eviction policy: remove oldest transaction
		evicted := tp.evictTransaction()
		if types.IsZeroHash(evicted) {
			return errors.New("transaction pool is full and eviction failed")
		}
	}

	// Add transaction with current timestamp
	tx.Timestamp = time.Now().UnixNano()
	tp.transactions[txID] = tx
	return nil
}

// evictTransaction removes the oldest transaction based on timestamp.
// Returns the evicted transaction ID.
func (tp *TxPool) evictTransaction() types.Hash {
	var oldestTxID types.Hash
	var oldestTimestamp int64 = math.MaxInt64

	for txID, tx := range tp.transactions {
		if tx.Timestamp < oldestTimestamp {
			oldestTimestamp = tx.Timestamp
			oldestTxID = txID
		}
	}

	if oldestTxID.String() != "" {
		delete(tp.transactions, oldestTxID)
		return oldestTxID
	}

	return types.ZeroHash
}

// GetTransactions retrieves up to 'max' transactions from the pool, sorted by nonce ascending.
func (tp *TxPool) GetTransactions(max int) []*types.Transaction {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	// Collect transactions into a slice
	txs := make([]*types.Transaction, 0, len(tp.transactions))
	for _, tx := range tp.transactions {
		txs = append(txs, tx)
	}

	// Sort transactions by nonce in ascending order
	sortTransactionsByNonceAsc(txs)

	// Return up to 'max' transactions
	if max > len(txs) {
		max = len(txs)
	}
	return txs[:max]
}

// sortTransactionsByNonceAsc sorts transactions in-place by their nonce in ascending order.
func sortTransactionsByNonceAsc(txs []*types.Transaction) {
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Nonce < txs[j].Nonce
	})
}

// sortTransactionsByFeeDesc sorts transactions in-place by their fee in descending order.
func sortTransactionsByFeeDesc(txs []*types.Transaction) {
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Fee > txs[j].Fee
	})
}

// RemoveTransactions removes transactions with the specified IDs from the pool.
func (tp *TxPool) RemoveTransactions(txIDs []types.Hash) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	for _, txID := range txIDs {
		delete(tp.transactions, txID)
	}
}

// RemoveTransaction removes transaction with the specified IDs from the pool.
func (tp *TxPool) RemoveTransaction(txID types.Hash) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	delete(tp.transactions, txID)
}

// GetTransaction retrieves a transaction by its ID.
// Returns the transaction and a boolean indicating its existence.
func (tp *TxPool) GetTransaction(txID types.Hash) (*types.Transaction, bool) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	tx, exists := tp.transactions[txID]
	return tx, exists
}

// Size returns the current number of transactions in the pool.
func (tp *TxPool) Size() int {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	return len(tp.transactions)
}

// cleanupExpiredTransactions periodically removes expired transactions.
func (tp *TxPool) cleanupExpiredTransactions() {
	ticker := time.NewTicker(tp.cleanupInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		now := time.Now().UnixNano()

		tp.mu.Lock()
		for txID, tx := range tp.transactions {
			if now-tx.Timestamp > tp.txExpiry.Nanoseconds() {
				delete(tp.transactions, txID)
			}
		}
		tp.mu.Unlock()
	}
}

// cleanupExpiredTransactionsWithInterval allows setting a custom cleanup interval.
func (tp *TxPool) cleanupExpiredTransactionsWithInterval(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		now := time.Now().UnixNano()

		tp.mu.Lock()
		for txID, tx := range tp.transactions {
			if now-tx.Timestamp > tp.txExpiry.Nanoseconds() {
				delete(tp.transactions, txID)
			}
		}
		tp.mu.Unlock()
	}
}
