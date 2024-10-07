package types

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/peerdns/peerdns/pkg/signatures"
)

// Block represents a block in the ledger with optimized fields and signatures.
type Block struct {
	Index         uint64                // Block index in the chain
	Timestamp     int64                 // Unix timestamp when the block was created
	PreviousHash  Hash                  // Hash of the previous block
	Hash          Hash                  // Current block's hash
	MerkleRoot    Hash                  // Merkle root of the transactions
	Signature     []byte                // Signature of the block
	SignatureType signatures.SignerType // Type of the signature
	ValidatorID   Address               // Validator's address who created the block
	Transactions  []*Transaction        // Transactions included in the block
	Difficulty    uint64                // Difficulty level for consensus
	Version       uint32                // Block version
	FinalizedBy   []Address             // Addresses of validators who finalized the block
}

// NewBlock creates a new Block, computes its Merkle root and hash.
func NewBlock(index uint64, previousHash Hash, transactions []*Transaction, validatorID Address, difficulty uint64) (*Block, error) {
	if len(transactions) == 0 {
		return nil, errors.New("block must contain at least one transaction")
	}

	block := &Block{
		Index:        index,
		Timestamp:    time.Now().Unix(),
		PreviousHash: previousHash,
		Transactions: transactions,
		ValidatorID:  validatorID,
		Difficulty:   difficulty,
		Version:      1, // Initial version
		Signature:    []byte{},
		FinalizedBy:  []Address{},
	}

	block.MerkleRoot = block.ComputeMerkleRoot()
	block.Hash = block.ComputeHash()

	return block, nil
}

// ComputeMerkleRoot computes the Merkle root of the block's transactions.
func (b *Block) ComputeMerkleRoot() Hash {
	var merkleRoot Hash

	txHashes := make([][]byte, len(b.Transactions))
	for i, tx := range b.Transactions {
		txHashes[i] = tx.ID.Bytes()
	}

	if len(txHashes) == 0 {
		return merkleRoot
	}

	for len(txHashes) > 1 {
		var nextLevel [][]byte

		for i := 0; i < len(txHashes); i += 2 {
			if i+1 < len(txHashes) {
				combined := append(txHashes[i], txHashes[i+1]...)
				hash := sha256.Sum256(combined)
				nextLevel = append(nextLevel, hash[:])
			} else {
				nextLevel = append(nextLevel, txHashes[i])
			}
		}

		txHashes = nextLevel
	}

	copy(merkleRoot[:], txHashes[0])

	return merkleRoot
}

// ComputeHash computes the SHA-256 hash of the block's contents.
func (b *Block) ComputeHash() Hash {
	var buffer []byte

	temp := make([]byte, 8)

	binary.LittleEndian.PutUint64(temp, b.Index)
	buffer = append(buffer, temp...)

	binary.LittleEndian.PutUint64(temp, uint64(b.Timestamp))
	buffer = append(buffer, temp...)

	buffer = append(buffer, b.PreviousHash.Bytes()...)
	buffer = append(buffer, b.MerkleRoot.Bytes()...)
	buffer = append(buffer, b.ValidatorID.Bytes()...)

	binary.LittleEndian.PutUint64(temp, b.Difficulty)
	buffer = append(buffer, temp...)

	binary.LittleEndian.PutUint32(temp[:4], b.Version)
	buffer = append(buffer, temp[:4]...)

	hash := sha256.Sum256(buffer)
	return hash
}

// Serialize serializes the block into a byte slice for storage or transmission.
func (b *Block) Serialize() ([]byte, error) {
	txCount := uint32(len(b.Transactions))
	txSize := 0
	txBytesList := make([][]byte, txCount) // Store serialized transactions

	// Serialize transactions and compute total size
	for i, tx := range b.Transactions {
		txBytes, err := tx.Serialize()
		if err != nil {
			return nil, err
		}
		txBytesList[i] = txBytes
		txSize += 4 + len(txBytes) // 4 bytes for length prefix + transaction data
	}

	finalizedCount := uint32(len(b.FinalizedBy))
	finalizedSize := int(finalizedCount) * AddressSize

	signatureLength := uint32(len(b.Signature))

	// Compute total size
	totalSize := 8 + 8 + HashSize*3 + 4 + 4 + int(signatureLength) + AddressSize + 8 + 4 + 4 + txSize + 4 + finalizedSize

	buf := make([]byte, totalSize)
	offset := 0

	binary.LittleEndian.PutUint64(buf[offset:], b.Index)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], uint64(b.Timestamp))
	offset += 8

	copy(buf[offset:], b.PreviousHash.Bytes())
	offset += HashSize

	copy(buf[offset:], b.Hash.Bytes())
	offset += HashSize

	copy(buf[offset:], b.MerkleRoot.Bytes())
	offset += HashSize

	// Write SignatureType as uint32
	binary.LittleEndian.PutUint32(buf[offset:], b.SignatureType.Uint32())
	offset += 4

	// Write Signature length
	binary.LittleEndian.PutUint32(buf[offset:], signatureLength)
	offset += 4

	// Write Signature data
	copy(buf[offset:], b.Signature)
	offset += int(signatureLength)

	// Write ValidatorID
	copy(buf[offset:], b.ValidatorID.Bytes())
	offset += AddressSize

	binary.LittleEndian.PutUint64(buf[offset:], b.Difficulty)
	offset += 8

	binary.LittleEndian.PutUint32(buf[offset:], b.Version)
	offset += 4

	// Write transaction count
	binary.LittleEndian.PutUint32(buf[offset:], txCount)
	offset += 4

	// Write transactions with length prefix
	for _, txBytes := range txBytesList {
		txLen := uint32(len(txBytes))
		binary.LittleEndian.PutUint32(buf[offset:], txLen)
		offset += 4
		copy(buf[offset:], txBytes)
		offset += int(txLen)
	}

	// Write finalized count
	binary.LittleEndian.PutUint32(buf[offset:], finalizedCount)
	offset += 4

	// Write FinalizedBy addresses
	for _, validatorAddr := range b.FinalizedBy {
		copy(buf[offset:], validatorAddr.Bytes())
		offset += AddressSize
	}

	return buf, nil
}

// DeserializeBlock deserializes a byte slice into a Block.
func DeserializeBlock(data []byte) (*Block, error) {
	minSize := 8 + 8 + HashSize*3 + 4 + 4 + AddressSize + 8 + 4 + 4
	if len(data) < minSize {
		return nil, errors.New("data too short to deserialize Block")
	}

	block := &Block{}
	offset := 0

	block.Index = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	block.Timestamp = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	block.PreviousHash, _ = HashFromBytes(data[offset : offset+HashSize])
	offset += HashSize

	block.Hash, _ = HashFromBytes(data[offset : offset+HashSize])
	offset += HashSize

	block.MerkleRoot, _ = HashFromBytes(data[offset : offset+HashSize])
	offset += HashSize

	// Read SignatureType
	if offset+4 > len(data) {
		return nil, errors.New("data too short to read SignatureType")
	}
	block.SignatureType = signatures.SignerTypeFromUint32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// Read Signature length
	if offset+4 > len(data) {
		return nil, errors.New("data too short to read Signature length")
	}
	signatureLength := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Read Signature data
	if offset+int(signatureLength) > len(data) {
		return nil, errors.New("data too short to read Signature data")
	}
	block.Signature = data[offset : offset+int(signatureLength)]
	offset += int(signatureLength)

	// Read ValidatorID
	if offset+AddressSize > len(data) {
		return nil, errors.New("data too short to read ValidatorID")
	}
	err := block.ValidatorID.UnmarshalBinary(data[offset : offset+AddressSize])
	if err != nil {
		return nil, err
	}
	offset += AddressSize

	if offset+8+4+4 > len(data) {
		return nil, errors.New("data too short to read Difficulty, Version, and Transaction count")
	}

	block.Difficulty = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	block.Version = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Read transaction count
	txCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	block.Transactions = make([]*Transaction, txCount)
	for i := uint32(0); i < txCount; i++ {
		if offset+4 > len(data) {
			return nil, errors.New("unexpected end of data while deserializing transaction length")
		}
		txLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(txLen) > len(data) {
			return nil, errors.New("unexpected end of data while deserializing transaction")
		}

		txData := data[offset : offset+int(txLen)]
		tx, err := DeserializeTransaction(txData)
		if err != nil {
			return nil, err
		}
		block.Transactions[i] = tx
		offset += int(txLen)
	}

	if offset+4 > len(data) {
		return nil, errors.New("data too short to deserialize FinalizedBy count")
	}

	finalizedCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	block.FinalizedBy = make([]Address, finalizedCount)
	for i := uint32(0); i < finalizedCount; i++ {
		if offset+AddressSize > len(data) {
			return nil, errors.New("data too short to deserialize FinalizedBy addresses")
		}
		err := block.FinalizedBy[i].UnmarshalBinary(data[offset : offset+AddressSize])
		if err != nil {
			return nil, err
		}
		offset += AddressSize
	}

	return block, nil
}

// Size returns the total size of the serialized Block.
func (b *Block) Size() int {
	txSize := 0
	for _, tx := range b.Transactions {
		txSize += 4 + tx.Size() // 4 bytes for length prefix + transaction size
	}
	finalizedSize := len(b.FinalizedBy) * AddressSize
	signatureLength := len(b.Signature)
	return 8 + 8 + HashSize*3 + 4 + 4 + signatureLength + AddressSize + 8 + 4 + 4 + txSize + 4 + finalizedSize
}
