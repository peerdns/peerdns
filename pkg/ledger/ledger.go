// pkg/ledger/ledger.go

package ledger

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/dominikbraun/graph"
	"github.com/pkg/errors"

	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"
)

// Ledger represents the DAG-based ledger responsible for managing transactions and states.
type Ledger struct {
	ctx     context.Context
	storage storage.Provider
	mutex   sync.RWMutex
	graph   graph.Graph[string, *types.Block]
}

// NewLedger creates a new Ledger instance with the given storage provider.
func NewLedger(ctx context.Context, storage storage.Provider) *Ledger {
	// Create a new directed graph that uses block hashes as vertex identifiers.
	g := graph.New(
		func(block *types.Block) string {
			return hex.EncodeToString(block.Hash[:])
		},
		graph.Directed(),
	)

	return &Ledger{
		ctx:     ctx,
		storage: storage,
		graph:   g,
	}
}

// AddBlock adds a new block to the ledger, linking it to its predecessor block.
func (l *Ledger) AddBlock(block *types.Block) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	blockHash := hex.EncodeToString(block.Hash[:])
	prevHash := hex.EncodeToString(block.PreviousHash[:])

	// Add the block as a vertex in the graph.
	err := l.graph.AddVertex(block)
	if err != nil && !errors.Is(err, graph.ErrVertexAlreadyExists) {
		return fmt.Errorf("failed to add vertex to graph: %w", err)
	}

	// Link the block to its predecessor.
	zeroHash := hex.EncodeToString(make([]byte, types.HashSize))
	if prevHash != zeroHash {
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

// GetBlock retrieves a block from the ledger by its hash.
func (l *Ledger) GetBlock(hash string) (*types.Block, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

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

// ValidateChain validates the ledger chain to ensure there are no cycles.
func (l *Ledger) ValidateChain() (bool, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Attempt a topological sort to detect cycles.
	_, err := graph.TopologicalSort(l.graph)
	if err != nil {
		if err.Error() == "graph contains at least one cycle" {
			return false, fmt.Errorf("ledger contains cycles")
		}
		return false, fmt.Errorf("failed to perform topological sort: %w", err)
	}

	return true, nil
}

// ListBlocks lists all blocks in the ledger.
func (l *Ledger) ListBlocks() ([]*types.Block, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	blocksMap := make(map[string]*types.Block)

	adjacencyMap, err := l.graph.AdjacencyMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get adjacency map: %w", err)
	}

	// Collect all vertices from the adjacency map.
	for vertexID := range adjacencyMap {
		block, err := l.graph.Vertex(vertexID)
		if err == nil {
			blocksMap[vertexID] = block
		}
	}

	// Also collect vertices from the edges (in case of isolated vertices).
	for _, edges := range adjacencyMap {
		for _, edge := range edges {
			if _, exists := blocksMap[edge.Target]; !exists {
				block, err := l.graph.Vertex(edge.Target)
				if err == nil {
					blocksMap[edge.Target] = block
				}
			}
		}
	}

	// Convert map to slice.
	blocks := make([]*types.Block, 0, len(blocksMap))
	for _, block := range blocksMap {
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// TraverseSuccessors traverses all successors of a given block hash.
func (l *Ledger) TraverseSuccessors(hash string) ([]*types.Block, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	var successors []*types.Block

	visited := make(map[string]bool)

	err := graph.BFS(l.graph, hash, func(u string) bool {
		if u == hash {
			return false
		}
		if visited[u] {
			return false
		}
		visited[u] = true

		block, err := l.graph.Vertex(u)
		if err != nil {
			return true // Stop traversal on error
		}
		successors = append(successors, block)
		return false
	})

	if err != nil {
		return nil, fmt.Errorf("failed to traverse successors: %w", err)
	}

	return successors, nil
}

// GetTransaction retrieves a transaction by its ID.
func (l *Ledger) GetTransaction(txID string) (*types.Transaction, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Iterate through blocks to find the transaction.
	blocks, err := l.ListBlocks()
	if err != nil {
		return nil, fmt.Errorf("failed to list blocks: %w", err)
	}

	for _, block := range blocks {
		for _, tx := range block.Transactions {
			if hex.EncodeToString(tx.ID[:]) == txID {
				return tx, nil
			}
		}
	}

	return nil, fmt.Errorf("transaction %s not found", txID)
}

// IterateTransactions applies a given function to all transactions.
func (l *Ledger) IterateTransactions(fn func(tx *types.Transaction) error) error {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	blocks, err := l.ListBlocks()
	if err != nil {
		return fmt.Errorf("failed to list blocks: %w", err)
	}

	for _, block := range blocks {
		for _, tx := range block.Transactions {
			if err := fn(tx); err != nil {
				return err
			}
		}
	}

	return nil
}

// SeekTransactions finds transactions matching a predicate.
func (l *Ledger) SeekTransactions(predicate func(tx *types.Transaction) bool) ([]*types.Transaction, error) {
	var matchedTxs []*types.Transaction

	err := l.IterateTransactions(func(tx *types.Transaction) error {
		if predicate(tx) {
			matchedTxs = append(matchedTxs, tx)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return matchedTxs, nil
}

// Close gracefully shuts down the ledger and releases any resources.
func (l *Ledger) Close() error {
	return l.storage.Close()
}
