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
	// Handle the case for genesis block (PreviousHash is zero hash)
	if types.IsZeroHash(block.PreviousHash) {
		// Add the genesis block as a vertex in the graph.
		if err := l.graph.AddVertex(block); err != nil && !errors.Is(err, graph.ErrVertexAlreadyExists) {
			return fmt.Errorf("failed to add genesis block to graph: %w", err)
		}

		// Serialize the genesis block and store it in the database.
		value, err := block.Serialize()
		if err != nil {
			return fmt.Errorf("failed to serialize genesis block: %w", err)
		}
		if err := l.storage.Set(block.Hash.Bytes(), value); err != nil {
			return fmt.Errorf("failed to store genesis block in database: %w", err)
		}

		// Apply the initial state for genesis block allocations.
		for _, tx := range block.Transactions {
			l.state.UpdateBalance(tx.Recipient, int64(tx.Amount))
		}

		// No need to add an edge or perform a cycle check for the genesis block.
		return nil
	}

	// Update the state with transactions in the block.
	if err := l.applyBlock(block); err != nil {
		return fmt.Errorf("failed to apply block to state: %w", err)
	}

	// Add the block as a vertex in the graph.
	err := l.graph.AddVertex(block)
	if err != nil && !errors.Is(err, graph.ErrVertexAlreadyExists) {
		return fmt.Errorf("failed to add vertex to graph: %w", err)
	}

	// Check if adding this edge will create a cycle.
	createsCycle, err := graph.CreatesCycle(l.graph, block.PreviousHash, block.Hash)
	if err != nil {
		return fmt.Errorf("failed to check for cycles: %w", err)
	}
	if createsCycle {
		return fmt.Errorf("adding block %s would create a cycle", block.Hash)
	}

	// Link the block to its predecessor.
	err = l.graph.AddEdge(block.PreviousHash, block.Hash)
	if err != nil && !errors.Is(err, graph.ErrEdgeAlreadyExists) {
		return fmt.Errorf("failed to add edge from %s to %s: %w", block.PreviousHash, block.Hash, err)
	}

	return nil
}

// GetBlock retrieves a block from the ledger by its hash.
func (l *Ledger) GetBlock(hash types.Hash) (*types.Block, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.getBlock(hash)
}

// getBlock is an internal method that retrieves a block without acquiring a lock.
func (l *Ledger) getBlock(hash types.Hash) (*types.Block, error) {
	// Retrieve the block from the graph.
	block, err := l.graph.Vertex(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block from graph: %w", err)
	}

	return block, nil
}

// GetBlockByIndex retrieves a block by its index.
func (l *Ledger) GetBlockByIndex(index uint64) (*types.Block, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Traverse through the graph to find the block with the given index.
	adjacencyMap, err := l.graph.AdjacencyMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get adjacency map: %w", err)
	}

	for _, edges := range adjacencyMap {
		for _, edge := range edges {
			block, err := l.graph.Vertex(edge.Target)
			if err == nil && block.Index == index {
				return block, nil
			}
		}
	}

	return nil, fmt.Errorf("block with index %d not found", index)
}

// GetLatestBlock retrieves the latest block in the chain.
func (l *Ledger) GetLatestBlock() (*types.Block, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Get the adjacency map of the graph.
	adjacencyMap, err := l.graph.AdjacencyMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get adjacency map: %w", err)
	}

	// Iterate through the blocks and find the one without successors (latest block).
	for blockHash, edges := range adjacencyMap {
		if len(edges) == 0 { // No successors, so it must be the latest block.
			block, err := l.graph.Vertex(blockHash)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve latest block from graph: %w", err)
			}
			return block, nil
		}
	}

	return nil, fmt.Errorf("failed to find the latest block")
}
