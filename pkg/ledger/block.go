package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Block represents the structure of a block in the Hybrid-Hash Tree ledger.
type Block struct {
	Index        uint64        // Position of the block in the chain.
	Timestamp    int64         // Timestamp when the block was created.
	PreviousHash string        // Hash of the previous block in the chain.
	Hash         string        // Hash of the current block.
	Nonce        uint64        // Nonce used in Proof-of-Work for finding a valid block.
	MerkleRoot   string        // Merkle root of the transactions in the block.
	Signature    []byte        // Digital signature of the block.
	ValidatorID  string        // ID of the validator or node that created the block.
	Difficulty   uint64        // Difficulty level of the current block (for PoW).
	Version      uint32        // Version number of the block for backward compatibility.
	Transactions []Transaction // List of transactions included in the block.
}

// NewBlock creates a new block with the given parameters.
func NewBlock(index uint64, previousHash string, transactions []Transaction, validatorID string, signature []byte, difficulty uint64) *Block {
	block := &Block{
		Index:        index,
		Timestamp:    time.Now().Unix(),
		PreviousHash: previousHash,
		ValidatorID:  validatorID,
		Signature:    signature,
		Difficulty:   difficulty,
		Version:      1, // Initial version
		Transactions: transactions,
	}

	// Calculate Merkle Root from transactions
	block.MerkleRoot = CalculateMerkleRoot(transactions)

	// Calculate block hash
	block.Hash = block.CalculateHash()

	return block
}

// CalculateHash computes the hash of the block based on its contents.
func (b *Block) CalculateHash() string {
	hashInput := b.PreviousHash + b.MerkleRoot + string(b.Timestamp) + b.ValidatorID
	hashBytes := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hashBytes[:])
}

// CalculateMerkleRoot calculates the Merkle root hash of the block's transactions.
func CalculateMerkleRoot(transactions []Transaction) string {
	var txHashes [][]byte

	// Get hash of each transaction
	for _, tx := range transactions {
		txHash := sha256.Sum256(tx.Serialize())
		txHashes = append(txHashes, txHash[:])
	}

	return calculateMerkleRootRecursive(txHashes)
}

// calculateMerkleRootRecursive computes the Merkle root recursively.
func calculateMerkleRootRecursive(hashes [][]byte) string {
	if len(hashes) == 1 {
		return hex.EncodeToString(hashes[0])
	}

	var newLevel [][]byte
	for i := 0; i < len(hashes); i += 2 {
		if i+1 == len(hashes) { // Handle odd number of hashes
			newLevel = append(newLevel, hashes[i])
		} else {
			combined := append(hashes[i], hashes[i+1]...)
			newHash := sha256.Sum256(combined)
			newLevel = append(newLevel, newHash[:])
		}
	}

	return calculateMerkleRootRecursive(newLevel)
}
