package ledger

import (
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/pkg/errors"
)

// TransactionManager handles creation and management of transactions in the ledger.
type TransactionManager struct {
	storage storage.Provider // Storage provider for transaction data
}

// NewTransactionManager initializes a new TransactionManager instance.
func NewTransactionManager(provider storage.Provider) *TransactionManager {
	return &TransactionManager{
		storage: provider,
	}
}

// CreateTransaction creates a new transaction and stores it in the database.
func (tm *TransactionManager) CreateTransaction(sender, receiver string, value uint64, signature []byte) (*Transaction, error) {
	// Create a new transaction
	tx := NewTransaction(sender, receiver, value, signature)

	// Store the transaction in the database using its hash as the key
	txKey := []byte(tx.Hash)
	txData := tx.Serialize()

	if err := tm.storage.Set(txKey, txData); err != nil {
		return nil, errors.Wrap(err, "failed to store transaction in database")
	}

	return tx, nil
}

// GetTransaction retrieves a transaction from the database by its hash.
func (tm *TransactionManager) GetTransaction(hash string) (*Transaction, error) {
	txData, err := tm.storage.Get([]byte(hash))
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve transaction from database")
	}

	// Deserialize the transaction data into a Transaction object
	tx, err := DeserializeTransaction(txData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to deserialize transaction data")
	}

	return tx, nil
}

// ValidateTransaction validates a transaction's integrity and digital signature.
func (tm *TransactionManager) ValidateTransaction(tx *Transaction) error {
	// Recalculate the transaction hash and compare with the stored hash
	calculatedHash := tx.CalculateHash()
	if calculatedHash != tx.Hash {
		return errors.New("transaction hash mismatch")
	}

	// Validate the digital signature (this can be expanded based on the cryptographic scheme)
	// Note: Add actual signature verification logic here.
	return nil
}
