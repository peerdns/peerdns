// pkg/types/transaction.go
package types

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/peerdns/peerdns/pkg/encryption"
)

// Transaction represents a transaction with fixed-size fields and T-BLS signatures.
type Transaction struct {
	ID        [HashSize]byte      // Transaction ID (SHA-256 hash)
	Sender    [AddressSize]byte   // Sender's address
	Recipient [AddressSize]byte   // Recipient's address
	Amount    uint64              // Amount being transferred
	Fee       uint64              // Transaction fee
	Nonce     uint64              // Unique nonce to prevent replay attacks
	Timestamp int64               // Unix timestamp of the transaction
	Signature [SignatureSize]byte // T-BLS signature
	Payload   []byte              // Additional data or payload
}

// NewTransaction creates a new Transaction and computes its ID.
func NewTransaction(sender, recipient [AddressSize]byte, amount, fee, nonce uint64, payload []byte) (*Transaction, error) {
	if len(payload) > MaximumPayloadSize {
		return nil, errors.New("payload size exceeds maximum allowed")
	}

	tx := &Transaction{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Fee:       fee,
		Nonce:     nonce,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	hash := tx.computeHash() // Assign to a variable first
	copy(tx.ID[:], hash[:])  // Slice the array

	return tx, nil
}

// computeHash computes the SHA-256 hash of the transaction's contents.
func (tx *Transaction) computeHash() [HashSize]byte {
	var buffer []byte
	buffer = append(buffer, tx.Sender[:]...)
	buffer = append(buffer, tx.Recipient[:]...)
	temp := make([]byte, 8)
	binary.LittleEndian.PutUint64(temp, tx.Amount)
	buffer = append(buffer, temp...)
	binary.LittleEndian.PutUint64(temp, tx.Fee)
	buffer = append(buffer, temp...)
	binary.LittleEndian.PutUint64(temp, tx.Nonce)
	buffer = append(buffer, temp...)
	binary.LittleEndian.PutUint64(temp, uint64(tx.Timestamp))
	buffer = append(buffer, temp...)
	buffer = append(buffer, tx.Payload...)

	hash := sha256Sum(buffer)
	return hash
}

// sha256Sum returns the SHA-256 hash of the input data.
func sha256Sum(data []byte) [HashSize]byte {
	return sha256.Sum256(data)
}

// Serialize serializes the transaction into a byte slice for storage or transmission.
func (tx *Transaction) Serialize() ([]byte, error) {
	buf := make([]byte, tx.Size())
	offset := 0

	copy(buf[offset:], tx.ID[:])
	offset += HashSize

	copy(buf[offset:], tx.Sender[:])
	offset += AddressSize

	copy(buf[offset:], tx.Recipient[:])
	offset += AddressSize

	binary.LittleEndian.PutUint64(buf[offset:], tx.Amount)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], tx.Fee)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], tx.Nonce)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], uint64(tx.Timestamp))
	offset += 8

	copy(buf[offset:], tx.Signature[:])
	offset += SignatureSize

	copy(buf[offset:], tx.Payload)

	return buf, nil
}

// DeserializeTransaction deserializes a byte slice into a Transaction.
func DeserializeTransaction(data []byte) (*Transaction, error) {
	minSize := HashSize + 2*AddressSize + 3*8 + SignatureSize
	if len(data) < minSize {
		return nil, errors.New("data too short to deserialize Transaction")
	}

	tx := &Transaction{}
	offset := 0

	copy(tx.ID[:], data[offset:offset+HashSize])
	offset += HashSize

	copy(tx.Sender[:], data[offset:offset+AddressSize])
	offset += AddressSize

	copy(tx.Recipient[:], data[offset:offset+AddressSize])
	offset += AddressSize

	tx.Amount = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	tx.Fee = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	tx.Nonce = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	tx.Timestamp = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	copy(tx.Signature[:], data[offset:offset+SignatureSize])
	offset += SignatureSize

	if offset < len(data) {
		tx.Payload = make([]byte, len(data)-offset)
		copy(tx.Payload, data[offset:])
	} else {
		tx.Payload = []byte{}
	}

	return tx, nil
}

// Size returns the total size of the serialized Transaction.
func (tx *Transaction) Size() int {
	return HashSize + 2*AddressSize + 3*8 + SignatureSize + len(tx.Payload)
}

// Sign signs the transaction using the provided BLS private key.
func (tx *Transaction) Sign(privateKey *encryption.BLSPrivateKey) error {
	hash := tx.computeHash()                               // Assign to a variable first
	signature, err := encryption.Sign(hash[:], privateKey) // Slice the array
	if err != nil {
		return err
	}
	if len(signature.Signature) != SignatureSize {
		return errors.New("invalid signature size")
	}
	copy(tx.Signature[:], signature.Signature[:]) // Slice the array
	return nil
}

// Verify verifies the transaction's signature using the provided BLS public key.
func (tx *Transaction) Verify(publicKey *encryption.BLSPublicKey) (bool, error) {
	signature := &encryption.BLSSignature{Signature: tx.Signature[:]} // Slice the array
	hash := tx.computeHash()                                          // Assign to a variable first
	return encryption.Verify(hash[:], signature, publicKey)           // Slice the array
}
