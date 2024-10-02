package ledger

import (
	"bytes"
	"encoding/gob"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/pkg/errors"
)

// Ledger represents the unified interface to the ledger system, combining block, transaction, and state management.
type Ledger struct {
	storage        storage.Provider // Storage provider interface (e.g., MDBX)
	currentBlock   *Block           // Pointer to the current block being processed
	previousBlocks []*Block         // Array of previous blocks for historical purposes
}

// NewLedger initializes a new Ledger instance with the given storage provider.
func NewLedger(provider storage.Provider) *Ledger {
	return &Ledger{
		storage:        provider,
		previousBlocks: []*Block{}, // Initialize with an empty block list
	}
}

// CreateGenesisBlock initializes the ledger with a genesis block.
func (l *Ledger) CreateGenesisBlock() (*Block, error) {
	genesisBlock := NewBlock(
		0,               // Index
		"",              // PreviousHash
		[]Transaction{}, // Empty transaction list for genesis block
		"GENESIS",       // ValidatorID for the genesis block
		nil,             // No signature for genesis block
		0,               // Difficulty
	)
	genesisBlock.Hash = genesisBlock.CalculateHash()

	// Serialize and store the block in the database
	if err := l.storeBlock(genesisBlock); err != nil {
		return nil, errors.Wrap(err, "failed to store genesis block")
	}

	// Update current block pointer to genesis block
	l.currentBlock = genesisBlock
	return genesisBlock, nil
}

// AddBlock creates and adds a new block to the ledger.
func (l *Ledger) AddBlock(transactions []Transaction, validatorID string, difficulty uint64) (*Block, error) {
	if l.currentBlock == nil {
		return nil, errors.New("no genesis block found, create a genesis block first")
	}

	// Create a new block
	newBlock := NewBlock(
		l.currentBlock.Index+1, // Increment index
		l.currentBlock.Hash,    // Set previous hash as the hash of the current block
		transactions,           // Include the new transactions
		validatorID,            // Validator ID for this block
		nil,                    // Signature will be added after creation
		difficulty,             // Block difficulty
	)

	// Calculate the hash for the new block
	newBlock.Hash = newBlock.CalculateHash()

	// Serialize and store the block in the database
	if err := l.storeBlock(newBlock); err != nil {
		return nil, errors.Wrap(err, "failed to store new block")
	}

	// Update pointers and block lists
	l.previousBlocks = append(l.previousBlocks, l.currentBlock)
	l.currentBlock = newBlock

	return newBlock, nil
}

// storeBlock serializes and stores a block in the MDBX database.
func (l *Ledger) storeBlock(block *Block) error {
	blockData, err := block.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block")
	}

	// Use the block hash as the key for storing the block data
	blockKey := []byte(block.Hash)

	// Store the block in the database
	if err := l.storage.Set(blockKey, blockData); err != nil {
		return errors.Wrap(err, "failed to store block in database")
	}

	return nil
}

// GetBlock retrieves a block from the database by its hash.
func (l *Ledger) GetBlock(hash string) (*Block, error) {
	blockData, err := l.storage.Get([]byte(hash))
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve block from database")
	}

	// Deserialize the block data into a Block object
	block, err := DeserializeBlock(blockData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block data")
	}

	return block, nil
}

// Serialize serializes the block for storage or transmission.
func (b *Block) Serialize() ([]byte, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(b)
	return buffer.Bytes(), err
}

// DeserializeBlock deserializes a byte slice into a Block.
func DeserializeBlock(data []byte) (*Block, error) {
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)
	return &block, err
}
