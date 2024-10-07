// pkg/ledger/traversal.go

package ledger

import (
	"fmt"

	"github.com/dominikbraun/graph"

	"github.com/peerdns/peerdns/pkg/types"
)

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
