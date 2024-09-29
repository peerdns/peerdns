// pkg/chain/blockchain.go
package chain

import (
	"sync"

	"github.com/peerdns/peerdns/pkg/storage"
	"go.uber.org/zap"
)

type Blockchain struct {
	storage *storage.Db
	logger  *zap.Logger
	mu      sync.RWMutex
	// Add other necessary fields, e.g., chain state, blocks
}

func NewBlockchain(db *storage.Db, logger *zap.Logger) *Blockchain {
	return &Blockchain{
		storage: db,
		logger:  logger,
	}
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Serialize and store the block
	data, err := block.Serialize()
	if err != nil {
		return err
	}
	err = bc.storage.Set(block.Hash, data)
	if err != nil {
		return err
	}

	bc.logger.Info("Block added to blockchain", zap.String("BlockHash", string(block.Hash)))
	return nil
}

// GetBlock retrieves a block by its hash
func (bc *Blockchain) GetBlock(hash []byte) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	data, err := bc.storage.Get(hash)
	if err != nil {
		return nil, err
	}

	block, err := DeserializeBlock(data)
	if err != nil {
		return nil, err
	}

	return block, nil
}
