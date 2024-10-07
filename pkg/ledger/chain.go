// pkg/ledger/chain.go

package ledger

import (
	"fmt"

	"github.com/dominikbraun/graph"
)

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
