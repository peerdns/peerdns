package types

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/peerdns/peerdns/pkg/signatures"
	"time"
)

// Transaction represents a transaction with fixed-size fields and signatures.
type Transaction struct {
	ID            Hash                  // Transaction ID (SHA-256 hash)
	Sender        Address               // Sender's address
	Recipient     Address               // Recipient's address
	Amount        uint64                // Amount being transferred
	Fee           uint64                // Transaction fee
	Nonce         uint64                // Unique nonce to prevent replay attacks
	Timestamp     int64                 // Unix timestamp of the transaction
	Signature     []byte                // Signature of the transaction
	SignatureType signatures.SignerType // Type of the signature
	Payload       []byte                // Additional data or payload
}

// NewTransaction creates a new Transaction and computes its ID.
func NewTransaction(sender, recipient Address, amount, fee, nonce uint64, payload []byte) (*Transaction, error) {
	tx := &Transaction{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Fee:       fee,
		Nonce:     nonce,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	tx.ID = tx.ComputeHash()

	return tx, nil
}

// ComputeHash computes the SHA-256 hash of the transaction's contents.
func (tx *Transaction) ComputeHash() Hash {
	var buffer []byte

	buffer = append(buffer, tx.Sender.Bytes()...)
	buffer = append(buffer, tx.Recipient.Bytes()...)

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

	hash := sha256.Sum256(buffer)
	return hash
}

// Serialize serializes the transaction into a byte slice for storage or transmission.
func (tx *Transaction) Serialize() ([]byte, error) {
	signatureLength := uint32(len(tx.Signature))
	payloadLength := uint32(len(tx.Payload))

	// Corrected totalSize calculation
	totalSize := HashSize + AddressSize*2 + 8*4 + 4 + 4 + signatureLength + 4 + payloadLength

	buf := make([]byte, totalSize)
	offset := 0

	// Write Transaction ID
	copy(buf[offset:], tx.ID.Bytes())
	offset += HashSize

	// Write Sender
	copy(buf[offset:], tx.Sender.Bytes())
	offset += AddressSize

	// Write Recipient
	copy(buf[offset:], tx.Recipient.Bytes())
	offset += AddressSize

	// Write Amount
	binary.LittleEndian.PutUint64(buf[offset:], tx.Amount)
	offset += 8

	// Write Fee
	binary.LittleEndian.PutUint64(buf[offset:], tx.Fee)
	offset += 8

	// Write Nonce
	binary.LittleEndian.PutUint64(buf[offset:], tx.Nonce)
	offset += 8

	// Write Timestamp
	binary.LittleEndian.PutUint64(buf[offset:], uint64(tx.Timestamp))
	offset += 8

	// Write SignatureType
	binary.LittleEndian.PutUint32(buf[offset:], tx.SignatureType.Uint32())
	offset += 4

	// Write Signature Length
	binary.LittleEndian.PutUint32(buf[offset:], signatureLength)
	offset += 4

	// Write Signature Data
	copy(buf[offset:], tx.Signature)
	offset += int(signatureLength)

	// Write Payload Length
	binary.LittleEndian.PutUint32(buf[offset:], payloadLength)
	offset += 4

	// Write Payload Data
	copy(buf[offset:], tx.Payload)
	offset += int(payloadLength)

	return buf, nil
}

// DeserializeTransaction deserializes a byte slice into a Transaction.
func DeserializeTransaction(data []byte) (*Transaction, error) {
	minSize := HashSize + AddressSize*2 + 8*4 + 4 + 4 + 4
	if len(data) < minSize {
		return nil, errors.New("data too short to deserialize Transaction")
	}

	tx := &Transaction{}
	offset := 0

	// Read Transaction ID
	var err error
	tx.ID, err = HashFromBytes(data[offset : offset+HashSize])
	if err != nil {
		return nil, err
	}
	offset += HashSize

	// Read Sender
	err = tx.Sender.UnmarshalBinary(data[offset : offset+AddressSize])
	if err != nil {
		return nil, err
	}
	offset += AddressSize

	// Read Recipient
	err = tx.Recipient.UnmarshalBinary(data[offset : offset+AddressSize])
	if err != nil {
		return nil, err
	}
	offset += AddressSize

	// Read Amount
	tx.Amount = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Read Fee
	tx.Fee = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Read Nonce
	tx.Nonce = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Read Timestamp
	tx.Timestamp = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	// Read SignatureType
	if offset+4 > len(data) {
		return nil, errors.New("data too short to read SignatureType")
	}
	tx.SignatureType = signatures.SignerTypeFromUint32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// Read Signature Length
	if offset+4 > len(data) {
		return nil, errors.New("data too short to read Signature length")
	}
	signatureLength := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Read Signature Data
	if offset+int(signatureLength) > len(data) {
		return nil, errors.New("data too short to read Signature data")
	}
	tx.Signature = data[offset : offset+int(signatureLength)]
	offset += int(signatureLength)

	// Read Payload Length
	if offset+4 > len(data) {
		return nil, errors.New("data too short to read Payload length")
	}
	payloadLength := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Read Payload Data
	if offset+int(payloadLength) > len(data) {
		return nil, errors.New("data too short to read Payload data")
	}
	tx.Payload = data[offset : offset+int(payloadLength)]
	offset += int(payloadLength)

	return tx, nil
}

// Size returns the total size of the serialized Transaction.
func (tx *Transaction) Size() int {
	signatureLength := len(tx.Signature)
	payloadLength := len(tx.Payload)
	return HashSize + AddressSize*2 + 8*4 + 4 + signatureLength + 4 + payloadLength
}
