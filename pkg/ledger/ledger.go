// pkg/ledger/ledger.go

package ledger

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/dominikbraun/graph"

	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"
)

// Ledger represents the DAG-based ledger responsible for managing transactions and states.
type Ledger struct {
	ctx     context.Context
	storage storage.Provider
	mutex   sync.RWMutex
	graph   graph.Graph[string, *types.Block]
	state   *State // State management
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
		state:   NewState(), // Initialize state
	}
}

// Close gracefully shuts down the ledger and releases any resources.
func (l *Ledger) Close() error {
	return l.storage.Close()
}
