package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"time"
)

// Transaction represents a basic transaction structure in the ledger.
type Transaction struct {
	Hash      string // Hash of the transaction.
	Sender    string // Address of the sender.
	Receiver  string // Address of the receiver.
	Value     uint64 // Value transferred in the transaction.
	Timestamp int64  // Timestamp of the transaction.
	Signature []byte // Digital signature of the transaction.
}

// NewTransaction creates a new transaction with the given parameters.
func NewTransaction(sender, receiver string, value uint64, signature []byte) *Transaction {
	tx := &Transaction{
		Sender:    sender,
		Receiver:  receiver,
		Value:     value,
		Timestamp: time.Now().Unix(),
		Signature: signature,
	}

	// Calculate transaction hash
	tx.Hash = tx.CalculateHash()
	return tx
}

// CalculateHash computes the hash of the transaction based on its contents.
func (tx *Transaction) CalculateHash() string {
	hashInput := tx.Sender + tx.Receiver + string(tx.Value) + string(tx.Timestamp)
	hashBytes := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hashBytes[:])
}

// Serialize serializes the transaction into a byte slice for storage or transmission.
func (tx *Transaction) Serialize() []byte {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	_ = encoder.Encode(tx)
	return buffer.Bytes()
}

// DeserializeTransaction deserializes a byte slice into a Transaction.
func DeserializeTransaction(data []byte) (*Transaction, error) {
	var tx Transaction
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&tx)
	return &tx, err
}
