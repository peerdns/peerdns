// pkg/types/block.go
package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/peerdns/peerdns/pkg/encryption"
)

// Block represents a block in the ledger with optimized fields and T-BLS signatures.
type Block struct {
	Index        uint64              // Block index in the chain
	Timestamp    int64               // Unix timestamp when the block was created
	PreviousHash [HashSize]byte      // Hash of the previous block
	Hash         [HashSize]byte      // Current block's hash
	MerkleRoot   [HashSize]byte      // Merkle root of the transactions
	Signature    [SignatureSize]byte // T-BLS signature of the block
	ValidatorID  [AddressSize]byte   // Validator's address who created the block
	Transactions []Transaction       // Transactions included in the block
	Difficulty   uint64              // Difficulty level for consensus
	Version      uint32              // Block version
	FinalizedBy  [][AddressSize]byte // Addresses of validators who finalized the block
}

// NewBlock creates a new Block, computes its Merkle root and hash.
func NewBlock(index uint64, previousHash [HashSize]byte, transactions []Transaction, validatorID [AddressSize]byte, difficulty uint64) (*Block, error) {
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
	}

	block.MerkleRoot = block.computeMerkleRoot()
	block.Hash = block.computeHash()

	return block, nil
}

// computeMerkleRoot computes the Merkle root of the block's transactions.
func (b *Block) computeMerkleRoot() [HashSize]byte {
	var merkleRoot [HashSize]byte

	txHashes := make([][]byte, len(b.Transactions))
	for i, tx := range b.Transactions {
		txHashes[i] = tx.ID[:]
	}

	if len(txHashes) == 0 {
		return merkleRoot
	}

	for len(txHashes) > 1 {
		var nextLevel [][]byte

		for i := 0; i < len(txHashes); i += 2 {
			if i+1 < len(txHashes) {
				combined := append(txHashes[i], txHashes[i+1]...)
				hash := sha256Sum(combined)
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

// computeHash computes the SHA-256 hash of the block's contents.
func (b *Block) computeHash() [HashSize]byte {
	var buffer bytes.Buffer

	binary.Write(&buffer, binary.LittleEndian, b.Index)
	binary.Write(&buffer, binary.LittleEndian, b.Timestamp)
	buffer.Write(b.PreviousHash[:])
	buffer.Write(b.MerkleRoot[:])
	buffer.Write(b.ValidatorID[:])
	binary.Write(&buffer, binary.LittleEndian, b.Difficulty)
	binary.Write(&buffer, binary.LittleEndian, b.Version)

	hash := sha256Sum(buffer.Bytes())
	return hash
}

// Serialize serializes the block into a byte slice for storage or transmission.
func (b *Block) Serialize() ([]byte, error) {
	txCount := uint32(len(b.Transactions))
	txSize := 0
	for _, tx := range b.Transactions {
		txSize += tx.Size()
	}

	finalizedCount := uint32(len(b.FinalizedBy))
	finalizedSize := int(finalizedCount) * AddressSize

	totalSize := 8 + 8 + HashSize + HashSize + HashSize + SignatureSize + AddressSize + 8 + 4 + txSize + 4 + finalizedSize

	buf := make([]byte, totalSize)
	offset := 0

	binary.LittleEndian.PutUint64(buf[offset:], b.Index)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], uint64(b.Timestamp))
	offset += 8

	copy(buf[offset:], b.PreviousHash[:])
	offset += HashSize

	copy(buf[offset:], b.Hash[:])
	offset += HashSize

	copy(buf[offset:], b.MerkleRoot[:])
	offset += HashSize

	copy(buf[offset:], b.Signature[:])
	offset += SignatureSize

	copy(buf[offset:], b.ValidatorID[:])
	offset += AddressSize

	binary.LittleEndian.PutUint64(buf[offset:], b.Difficulty)
	offset += 8

	binary.LittleEndian.PutUint32(buf[offset:], txCount)
	offset += 4

	for _, tx := range b.Transactions {
		txBytes, err := tx.Serialize()
		if err != nil {
			return nil, err
		}
		copy(buf[offset:], txBytes)
		offset += tx.Size()
	}

	binary.LittleEndian.PutUint32(buf[offset:], finalizedCount)
	offset += 4

	for _, validatorAddr := range b.FinalizedBy {
		copy(buf[offset:], validatorAddr[:])
		offset += AddressSize
	}

	return buf, nil
}

// DeserializeBlock deserializes a byte slice into a Block.
func DeserializeBlock(data []byte) (*Block, error) {
	minSize := 8 + 8 + HashSize + HashSize + HashSize + SignatureSize + AddressSize + 8 + 4 + 4
	if len(data) < minSize {
		return nil, errors.New("data too short to deserialize Block")
	}

	block := &Block{}
	offset := 0

	block.Index = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	block.Timestamp = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8

	copy(block.PreviousHash[:], data[offset:offset+HashSize])
	offset += HashSize

	copy(block.Hash[:], data[offset:offset+HashSize])
	offset += HashSize

	copy(block.MerkleRoot[:], data[offset:offset+HashSize])
	offset += HashSize

	copy(block.Signature[:], data[offset:offset+SignatureSize])
	offset += SignatureSize

	copy(block.ValidatorID[:], data[offset:offset+AddressSize])
	offset += AddressSize

	block.Difficulty = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	txCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	block.Transactions = make([]Transaction, txCount)
	for i := uint32(0); i < txCount; i++ {
		if offset >= len(data) {
			return nil, errors.New("unexpected end of data while deserializing transactions")
		}

		tx, err := DeserializeTransaction(data[offset:])
		if err != nil {
			return nil, err
		}
		block.Transactions[i] = *tx
		offset += tx.Size()
	}

	if offset+4 > len(data) {
		return nil, errors.New("data too short to deserialize FinalizedBy count")
	}

	finalizedCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	block.FinalizedBy = make([][AddressSize]byte, finalizedCount)
	for i := uint32(0); i < finalizedCount; i++ {
		if offset+AddressSize > len(data) {
			return nil, errors.New("data too short to deserialize FinalizedBy addresses")
		}
		copy(block.FinalizedBy[i][:], data[offset:offset+AddressSize])
		offset += AddressSize
	}

	return block, nil
}

// Size returns the total size of the serialized Block.
func (b *Block) Size() int {
	txSize := 0
	for _, tx := range b.Transactions {
		txSize += tx.Size()
	}
	finalizedSize := len(b.FinalizedBy) * AddressSize
	return 8 + 8 + HashSize + HashSize + HashSize + SignatureSize + AddressSize + 8 + 4 + txSize + 4 + finalizedSize
}

// Sign signs the block using the provided BLS private key.
func (b *Block) Sign(privateKey *encryption.BLSPrivateKey) error {
	hash := b.computeHash()                                // Assign to a variable first
	signature, err := encryption.Sign(hash[:], privateKey) // Slice the array
	if err != nil {
		return err
	}
	if len(signature.Signature) != SignatureSize {
		return errors.New("invalid signature size")
	}
	copy(b.Signature[:], signature.Signature[:]) // Slice the array
	return nil
}

// Verify verifies the block's signature using the provided BLS public key.
func (b *Block) Verify(publicKey *encryption.BLSPublicKey) (bool, error) {
	signature := &encryption.BLSSignature{Signature: b.Signature[:]} // Slice the array
	hash := b.computeHash()                                          // Assign to a variable first
	return encryption.Verify(hash[:], signature, publicKey)          // Slice the array
}

// AddFinalizer adds a validator's address to the FinalizedBy list.
func (b *Block) AddFinalizer(validatorAddr [AddressSize]byte) {
	b.FinalizedBy = append(b.FinalizedBy, validatorAddr)
}
