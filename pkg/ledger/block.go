// pkg/ledger/block.go

package ledger

import (
	"fmt"

	"github.com/dominikbraun/graph"
	"github.com/pkg/errors"

	"github.com/peerdns/peerdns/pkg/types"
)

// AddBlock adds a new block to the ledger, linking it to its predecessor block.
func (l *Ledger) AddBlock(block *types.Block) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Validate the block before adding
	if err := l.ValidateBlock(block); err != nil {
		return fmt.Errorf("block validation failed: %w", err)
	}

	// Update the state with transactions in the block
	if err := l.applyBlock(block); err != nil {
		return fmt.Errorf("failed to apply block to state: %w", err)
	}

	// Add the block as a vertex in the graph.
	err := l.graph.AddVertex(block)
	if err != nil && !errors.Is(err, graph.ErrVertexAlreadyExists) {
		return fmt.Errorf("failed to add vertex to graph: %w", err)
	}

	// Link the block to its predecessor, unless it's the genesis block.
	zeroHash := types.Hash{} // Zero hash
	if block.PreviousHash != zeroHash {
		prevHash := block.PreviousHash.Hex()
		blockHash := block.Hash.Hex()

		// Check if adding this edge will create a cycle.
		createsCycle, err := graph.CreatesCycle(l.graph, prevHash, blockHash)
		if err != nil {
			return fmt.Errorf("failed to check for cycles: %w", err)
		}
		if createsCycle {
			return fmt.Errorf("adding block %s would create a cycle", blockHash)
		}

		err = l.graph.AddEdge(prevHash, blockHash)
		if err != nil && !errors.Is(err, graph.ErrEdgeAlreadyExists) {
			return fmt.Errorf("failed to add edge from %s to %s: %w", prevHash, blockHash, err)
		}
	}

	// Serialize the block and store it in the database.
	blockHash := block.Hash.Hex()
	key := []byte(blockHash) // Use hex string as key
	value, err := block.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize block: %w", err)
	}
	if err := l.storage.Set(key, value); err != nil {
		return fmt.Errorf("failed to store block in database: %w", err)
	}

	return nil
}

// GetBlockByHash retrieves a block by its types.Hash.
func (l *Ledger) GetBlockByHash(hash types.Hash) (*types.Block, error) {
	return l.getBlock(hash.Hex())
}

// GetBlock retrieves a block from the ledger by its hash.
func (l *Ledger) GetBlock(hash string) (*types.Block, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.getBlock(hash)
}

// getBlock is an internal method that retrieves a block without acquiring a lock.
func (l *Ledger) getBlock(hash string) (*types.Block, error) {
	// First, try to get the block from the graph.
	block, err := l.graph.Vertex(hash)
	if err == nil {
		return block, nil
	}

	// If not in graph, load from storage.
	key := []byte(hash) // Use hex string as key
	value, err := l.storage.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get block from database: %w", err)
	}

	block, err = types.DeserializeBlock(value)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize block: %w", err)
	}

	// Add the block to the graph for future use.
	err = l.graph.AddVertex(block)
	if err != nil && !errors.Is(err, graph.ErrVertexAlreadyExists) {
		return nil, fmt.Errorf("failed to add block to graph: %w", err)
	}

	return block, nil
}
