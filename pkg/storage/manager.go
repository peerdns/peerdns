package storage

import (
	"context"
	"fmt"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/types"
	"sync"
)

// Manager is responsible for managing multiple MDBX database instances based on the
// provided configuration. It allows easy access to different databases by name and
// handles the lifecycle operations such as opening, closing, and managing individual
// database instances.
//
// The Manager uses the MDBX configuration to set up the databases and manages the
// connections throughout the application's lifetime.
type Manager struct {
	// ctx represents the context used for managing the database lifecycle, such as
	// cancellation or timeouts.
	ctx context.Context

	// opts holds the MDBX configuration, which includes details such as the enabled status
	// and the nodes that define the MDBX instances.
	opts config.Mdbx

	// dbs is a map that holds the active MDBX databases, indexed by their DbType (name).
	dbs map[types.DbType]Provider

	// mutex for thread-safe operations on dbs
	mutex sync.RWMutex
}

// NewManager creates a new Manager instance that manages multiple MDBX database instances
// based on the configuration provided. It initializes the databases if MDBX is enabled and
// stores them in the Manager.
//
// Example usage:
//
//	mdbxManager, err := NewManager(ctx, config.Mdbx)
//	if err != nil {
//	    log.Fatalf("Failed to create MDBX manager: %v", err)
//	}
//
// Parameters:
//
//	ctx (context.Context): The context used for managing the database's lifecycle.
//	opts (config.Mdbx): The MDBX configuration specifying the database nodes.
//
// Returns:
//
//	*Manager: A new Manager instance that manages the MDBX databases.
//	error: Returns an error if any database initialization fails.
func NewManager(ctx context.Context, opts config.Mdbx) (*Manager, error) {
	dbs := make(map[types.DbType]Provider)
	if opts.Enabled {
		for _, node := range opts.Nodes {
			db, err := NewDb(ctx, node)
			if err != nil {
				return nil, err
			}
			// Store the database in the manager map, indexed by DbType (name).
			dbs[types.DbType(node.Name)] = db
		}
	}
	return &Manager{ctx: ctx, opts: opts, dbs: dbs}, nil
}

// CreateDb creates a new database with the given name.
func (m *Manager) CreateDb(name types.DbType) (*Db, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.dbs[name]; exists {
		return nil, fmt.Errorf("database %s already exists", name)
	}

	// Define default node configuration or fetch from opts
	// For simplicity, using default values
	node := config.MdbxNode{
		Name:            name.String(),
		Path:            fmt.Sprintf("./data/%s.mdbx", name),
		MaxReaders:      126,
		MaxSize:         1024,             // in GB
		MinSize:         1,                // in MB
		GrowthStep:      10 * 1024 * 1024, // 10 MB
		FilePermissions: 0600,
	}

	db, err := NewDb(m.ctx, node)
	if err != nil {
		return nil, err
	}
	m.dbs[name] = db

	// Perform type assertion before returning
	dbConcrete, ok := db.(*Db)
	if !ok {
		return nil, fmt.Errorf("database %s is not of type *Db", name)
	}

	return dbConcrete, nil
}

// GetDb retrieves a specific database by its name (DbType) from the manager.
// If the database is not found, an error is returned.
//
// Example usage:
//
//	db, err := mdbxManager.GetDb("node1")
//	if err != nil {
//	    log.Fatalf("Failed to retrieve MDBX database: %v", err)
//	}
//
// Parameters:
//
//	name (types.DbType): The name of the database to retrieve.
//
// Returns:
//
//	Provider: The database provider associated with the specified name.
//	error: Returns an error if the database is not found.
func (m *Manager) GetDb(name types.DbType) (Provider, error) {
	db, ok := m.dbs[name]
	if !ok {
		return nil, fmt.Errorf("mdbx database not found: %s", name)
	}
	return db, nil
}

// Close gracefully closes all managed databases in the Manager. It iterates through all
// the databases and calls their respective Close methods to ensure proper resource cleanup.
//
// Example usage:
//
//	err := mdbxManager.Close()
//	if err != nil {
//	    log.Fatalf("Failed to close MDBX manager: %v", err)
//	}
//
// Returns:
//
//	error: Returns an error if any of the databases fail to close properly.
func (m *Manager) Close() error {
	for _, db := range m.dbs {
		if err := db.Close(); err != nil {
			return err
		}
	}
	return nil
}
