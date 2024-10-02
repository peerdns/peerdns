// pkg/chain/blockchain.go
package chain

import (
	"github.com/peerdns/peerdns/pkg/logger"
	"sync"

	"github.com/peerdns/peerdns/pkg/storage"
)

type Blockchain struct {
	storage *storage.Db
	logger  logger.Logger
	mu      sync.RWMutex
	// Add other necessary fields, e.g., chain state, blocks
}

func NewBlockchain(db *storage.Db, logger logger.Logger) *Blockchain {
	return &Blockchain{
		storage: db,
		logger:  logger,
	}
}
