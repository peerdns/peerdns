package chain

import (
	"context"
	"fmt"
	"github.com/peerdns/peerdns/pkg/genesis"
	"github.com/peerdns/peerdns/pkg/ledger"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/types"
	"sync"
)

type Blockchain struct {
	ctx          context.Context
	logger       logger.Logger
	mu           sync.RWMutex
	ledger       *ledger.Ledger
	head         *types.Block
	genesisBlock *types.Block
}

// NewBlockchain initializes the blockchain with the provided context, logger, ledger, and genesis configuration.
func NewBlockchain(ctx context.Context, logger logger.Logger, ledger *ledger.Ledger, genesisConfig *genesis.Genesis) (*Blockchain, error) {
	bc := &Blockchain{
		ctx:    ctx,
		logger: logger,
		ledger: ledger,
	}

	// Initialize the blockchain
	if err := bc.setup(genesisConfig); err != nil {
		return nil, err
	}

	return bc, nil
}

// setup initializes the blockchain, ensuring the genesis block is present.
func (bc *Blockchain) setup(genesisConfig *genesis.Genesis) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Check if the ledger already has blocks
	latestBlock, err := bc.ledger.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	if latestBlock == nil {
		// No blocks found, create genesis block
		bc.logger.Info("No blocks found in ledger. Creating genesis block.")
		genesisBlock, err := genesis.CreateGenesisBlock(types.ZeroAddress, genesisConfig)
		if err != nil {
			return fmt.Errorf("failed to create genesis block: %w", err)
		}

		// Add genesis block to ledger
		if err := bc.ledger.AddBlock(genesisBlock); err != nil {
			return fmt.Errorf("failed to add genesis block to ledger: %w", err)
		}

		// Set the head and genesis block to the newly created genesis block
		bc.head = genesisBlock
		bc.genesisBlock = genesisBlock
	} else {
		// Ledger already has blocks, set the latest block as head
		bc.head = latestBlock

		// Find the genesis block by traversing the chain back to the beginning
		currentBlock := latestBlock
		for currentBlock.PreviousHash != types.ZeroHash {
			currentBlock, err = bc.ledger.GetBlock(currentBlock.PreviousHash)
			if err != nil {
				return fmt.Errorf("failed to traverse to genesis block: %w", err)
			}
		}
		// Set the genesis block
		bc.genesisBlock = currentBlock
	}

	return nil
}

// GetChainHeight returns the current height of the blockchain.
func (bc *Blockchain) GetChainHeight() (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.head == nil {
		return 0, ErrNoGenesisBlock
	}

	return bc.head.Index, nil
}

func (bc *Blockchain) ValidateChain() (bool, error) {
	return bc.ledger.ValidateChain()
}

func (bc *Blockchain) Ledger() *ledger.Ledger {
	return bc.ledger
}

func (bc *Blockchain) State() *ledger.State {
	return bc.ledger.State()
}
