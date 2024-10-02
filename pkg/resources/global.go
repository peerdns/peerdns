package resources

import (
	"github.com/peerdns/peerdns/pkg/observability"
	"sync"
)

var (
	// global is the global resource manager instance.
	global *Manager

	// once ensures that the global resource manager is only initialized once.
	once sync.Once
)

// G returns the global resource manager instance.
func G() *Manager {
	return global
}

// InitializeManager initializes the global resource manager if it hasn't been initialized already.
func InitializeManager(obs *observability.Observability) (*Manager, error) {
	once.Do(func() { global = NewManager(obs) })
	return global, nil
}
