// pkg/txpool/txpool_test.go

package txpool

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/peerdns/peerdns/pkg/signatures"
	"github.com/peerdns/peerdns/pkg/types"
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
func createTestTransaction(id string, nonce uint64, fee uint64) *types.Transaction {
	txID := sha256.Sum256([]byte(id))
	senderAddress := createAddress("did:peer:sender1")
	recipientAddress := createAddress("did:peer:recipient1")
	tx := &types.Transaction{
		ID:            txID,
		Sender:        senderAddress,
		Recipient:     recipientAddress,
		Amount:        1000,
		Fee:           fee,
		Nonce:         nonce,
		Timestamp:     time.Now().UnixNano(), // Changed to UnixNano
		Payload:       []byte("test payload"),
		Signature:     []byte{},                 // Initialize Signature
		SignatureType: signatures.BlsSignerType, // Initialize SignatureType
	}
	return tx
}

// Mock validation function for testing purposes.
// Allows us to simulate transaction validation without implementing actual logic.
func mockValidateTransaction(tx *types.Transaction) error {
	if tx.Amount == 0 {
		return errors.New("transaction amount must be greater than zero")
	}
	// tx.Fee is uint64, cannot be negative
	// Additional validations can be added here
	return nil
}

func TestTxPool(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(pool *TxPool) ([]*types.Transaction, []types.Hash)
		operation   func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error
		assertion   func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash)
		expectError bool
	}{
		{
			name: "AddUniqueValidTransactions",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				txs := []*types.Transaction{
					createTestTransaction("tx01", 1, 10),
					createTestTransaction("tx02", 2, 20),
					createTestTransaction("tx03", 3, 30),
				}
				var txIDs []types.Hash
				for _, tx := range txs {
					txIDs = append(txIDs, tx.ID)
				}
				return txs, txIDs
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				// Override the validateTransaction method for testing
				pool.validateTransaction = mockValidateTransaction
				defer func() { pool.validateTransaction = pool.validateTransactionDefault }()

				for _, tx := range txs {
					if err := pool.AddTransaction(tx); err != nil {
						return err
					}
				}
				return nil
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				assert.Equal(t, len(txs), pool.Size(), "Pool size should match number of added transactions")
				for _, tx := range txs {
					retrievedTx, exists := pool.GetTransaction(tx.ID)
					assert.True(t, exists, "Transaction should exist in the pool")
					assert.Equal(t, tx.ID, retrievedTx.ID, "Transaction IDs should match")
				}
			},
			expectError: false,
		},
		{
			name: "AddDuplicateTransactions",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				tx := createTestTransaction("tx04", 4, 40)
				return []*types.Transaction{tx}, []types.Hash{tx.ID}
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				pool.validateTransaction = mockValidateTransaction
				defer func() { pool.validateTransaction = pool.validateTransactionDefault }()

				// Add the same transaction twice
				if err := pool.AddTransaction(txs[0]); err != nil {
					return err
				}
				return pool.AddTransaction(txs[0]) // This should fail
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				assert.Equal(t, 1, pool.Size(), "Pool size should be 1 after adding duplicate")
			},
			expectError: true,
		},
		{
			name: "AddInvalidTransaction",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				// Transaction with invalid amount (Amount = 0)
				txInvalid := createTestTransaction("tx05_invalid", 5, 50)
				txInvalid.Amount = 0 // Invalid amount
				return []*types.Transaction{txInvalid}, []types.Hash{txInvalid.ID}
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				pool.validateTransaction = mockValidateTransaction
				defer func() { pool.validateTransaction = pool.validateTransactionDefault }()

				return pool.AddTransaction(txs[0]) // Should fail
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				assert.Equal(t, 0, pool.Size(), "Pool size should remain 0 after adding invalid transaction")
			},
			expectError: true,
		},
		{
			name: "GetTransactionsByFee",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				txs := []*types.Transaction{
					createTestTransaction("tx06", 6, 10), // Lower fee
					createTestTransaction("tx07", 7, 30), // Higher fee
					createTestTransaction("tx08", 8, 20),
				}
				var txIDs []types.Hash
				for _, tx := range txs {
					txIDs = append(txIDs, tx.ID)
				}
				return txs, txIDs
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				pool.validateTransaction = mockValidateTransaction
				defer func() { pool.validateTransaction = pool.validateTransactionDefault }()

				for _, tx := range txs {
					if err := pool.AddTransaction(tx); err != nil {
						return err
					}
				}
				return nil
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				retrievedTxs := pool.GetTransactions(3)
				assert.Len(t, retrievedTxs, 3, "Should retrieve 3 transactions")

				// Expected order: tx07 (fee=30), tx08 (fee=20), tx06 (fee=10)
				expectedFees := []uint64{30, 20, 10}
				for i, expectedFee := range expectedFees {
					assert.Equal(t, expectedFee, retrievedTxs[i].Fee, fmt.Sprintf("Transaction at position %d should have fee %d", i, expectedFee))
				}
			},
			expectError: false,
		},
		{
			name: "PoolCapacityEviction",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				txs := []*types.Transaction{}
				txIDs := []types.Hash{}
				// Create transactions to fill the pool to maxSize
				for i := 1; i <= pool.maxSize; i++ {
					tx := createTestTransaction(
						fmt.Sprintf("tx_cap_%d", i),
						uint64(i),
						uint64(i%100),
					)
					txs = append(txs, tx)
					txIDs = append(txIDs, tx.ID)
				}
				return txs, txIDs
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				pool.validateTransaction = mockValidateTransaction
				defer func() { pool.validateTransaction = pool.validateTransactionDefault }()

				for _, tx := range txs {
					if err := pool.AddTransaction(tx); err != nil {
						return err
					}
				}

				// Add one more transaction to trigger eviction
				txEvict := createTestTransaction("tx_evicted", uint64(pool.maxSize+1), 999)
				return pool.AddTransaction(txEvict)
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				assert.Equal(t, pool.maxSize, pool.Size(), "Pool size should remain at maxSize after eviction")

				// The first transaction (tx_cap_1) should have been evicted
				evictedTxID := txIDs[0]
				_, exists := pool.GetTransaction(evictedTxID)
				assert.False(t, exists, "Oldest transaction should have been evicted")

				// The new transaction should exist
				newTxID := createTestTransaction("tx_evicted", uint64(pool.maxSize+1), 999).ID
				tx, exists := pool.GetTransaction(newTxID)
				assert.True(t, exists, "Newly added transaction should exist in the pool")
				assert.Equal(t, newTxID, tx.ID, "New transaction ID should match")
			},
			expectError: false,
		},
		{
			name: "TransactionExpiry",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				// Create a TxPool with a short cleanup interval for testing
				customPool := NewTxPoolWithOptions(
					pool.maxSize,
					2*time.Second,           // txExpiry
					100*time.Millisecond,    // cleanupInterval
					mockValidateTransaction, // validateFn
				)

				// Replace the existing pool's fields with the custom pool's fields
				pool.mu.Lock()
				defer pool.mu.Unlock()
				pool.transactions = customPool.transactions
				pool.txExpiry = customPool.txExpiry
				pool.cleanupInterval = customPool.cleanupInterval
				pool.validateTransaction = customPool.validateTransaction

				txs := []*types.Transaction{
					createTestTransaction("tx09", 9, 90),
					createTestTransaction("tx10", 10, 100),
				}
				var txIDs []types.Hash
				for _, tx := range txs {
					txIDs = append(txIDs, tx.ID)
				}
				return txs, txIDs
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				for _, tx := range txs {
					if err := pool.AddTransaction(tx); err != nil {
						return err
					}
				}

				// Wait for transactions to expire
				time.Sleep(3 * time.Second)
				return nil
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				assert.Equal(t, 0, pool.Size(), "Pool should be empty after transaction expiry")
			},
			expectError: false,
		},
		{
			name: "ConcurrentAccess",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				txs := []*types.Transaction{}
				txIDs := []types.Hash{}
				for i := 1; i <= 100; i++ {
					tx := createTestTransaction(
						fmt.Sprintf("concurrent_tx_%d", i),
						uint64(i),
						uint64(i%100),
					)
					txs = append(txs, tx)
					txIDs = append(txIDs, tx.ID)
				}
				return txs, txIDs
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				var wg sync.WaitGroup
				// Add transactions concurrently
				for _, tx := range txs {
					wg.Add(1)
					go func(tx *types.Transaction) {
						defer wg.Done()
						pool.AddTransaction(tx)
					}(tx)
				}
				wg.Wait()

				// Remove transactions concurrently
				for _, txID := range txIDs[:50] { // Remove first 50
					wg.Add(1)
					go func(txID types.Hash) {
						defer wg.Done()
						pool.RemoveTransactions([]types.Hash{txID})
					}(txID)
				}
				wg.Wait()
				return nil
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				assert.Equal(t, 50, pool.Size(), "Pool size should be 50 after concurrent additions and removals")

				// Verify that the remaining transactions are tx51 to tx100
				for i := 51; i <= 100; i++ {
					txID := txIDs[i-1]
					tx, exists := pool.GetTransaction(txID)
					assert.True(t, exists, "Transaction should exist in the pool")
					assert.Equal(t, txID, tx.ID, "Transaction ID should match")
				}

				// Verify that transactions tx01 to tx50 have been removed
				for i := 1; i <= 50; i++ {
					txID := txIDs[i-1]
					_, exists := pool.GetTransaction(txID)
					assert.False(t, exists, "Transaction should have been removed from the pool")
				}
			},
			expectError: false,
		},
		{
			name: "GetTransactionsEmptyPool",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				// No transactions added
				return []*types.Transaction{}, []types.Hash{}
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				// No operations
				return nil
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				retrievedTxs := pool.GetTransactions(10)
				assert.Len(t, retrievedTxs, 0, "Should retrieve 0 transactions from an empty pool")
			},
			expectError: false,
		},
		{
			name: "RemoveTransactionsFromEmptyPool",
			setup: func(pool *TxPool) ([]*types.Transaction, []types.Hash) {
				// No transactions added
				return []*types.Transaction{}, []types.Hash{{00, 01}, {00, 02}}
			},
			operation: func(pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) error {
				// Attempt to remove transactions from an empty pool
				pool.RemoveTransactions(txIDs)
				return nil
			},
			assertion: func(t *testing.T, pool *TxPool, txs []*types.Transaction, txIDs []types.Hash) {
				assert.Equal(t, 0, pool.Size(), "Pool size should remain 0 after attempting to remove from an empty pool")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize the pool
			var pool *TxPool
			if tt.name == "TransactionExpiry" {
				// For TransactionExpiry test, use a custom pool with short cleanup interval
				pool = NewTxPoolWithOptions(
					DefaultMaxPoolSize,
					2*time.Second,           // txExpiry
					100*time.Millisecond,    // cleanupInterval
					mockValidateTransaction, // validateFn
				)
			} else {
				pool = NewTxPool()
			}

			// Setup transactions and txIDs
			txs, txIDs := tt.setup(pool)

			// Perform the operation
			err := tt.operation(pool, txs, txIDs)
			if tt.expectError {
				require.Error(t, err, "Expected an error but got none")
			} else {
				require.NoError(t, err, "Operation failed unexpectedly")
			}

			// Perform assertions
			tt.assertion(t, pool, txs, txIDs)
		})
	}
}
