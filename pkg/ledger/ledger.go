package ledger

import (
	"context"
	"sync"

	"github.com/dominikbraun/graph"

	"github.com/peerdns/peerdns/pkg/graphmdbx" // Import the graphmdbx package
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/peerdns/peerdns/pkg/types"
)

// Ledger represents the DAG-based ledger responsible for managing transactions and states.
type Ledger struct {
	ctx     context.Context
	storage storage.Provider
	mutex   sync.RWMutex
	graph   graph.Graph[types.Hash, *types.Block]
	state   *State // State management
}

// NewLedger creates a new Ledger instance with the given storage provider.
func NewLedger(ctx context.Context, storage storage.Provider) (*Ledger, error) {
	// Create a new graphmdbx store.
	vertexPrefix := "vertex"
	edgePrefix := "edge"
	graphStore := graphmdbx.New[types.Hash, *types.Block](storage, vertexPrefix, edgePrefix, ctx)

	// Create a new directed graph that uses block hashes as vertex identifiers.
	g := graph.NewWithStore(
		func(block *types.Block) types.Hash {
			return block.Hash
		},
		graphStore,
		graph.Directed(),
	)

	return &Ledger{
		ctx:     ctx,
		storage: storage,
		graph:   g,
		state:   NewState(), // Initialize state
	}, nil
}

func (l *Ledger) Graph() graph.Graph[types.Hash, *types.Block] {
	return l.graph
}

func (l *Ledger) State() *State {
	return l.state
}

func (l *Ledger) Storage() *storage.Provider {
	return &l.storage
}

// Close gracefully shuts down the ledger and releases any resources.
func (l *Ledger) Close() error {
	return l.storage.Close()
}
