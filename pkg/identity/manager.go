// pkg/identity/manager.go
package identity

import (
	"fmt"
	"sync"

	"github.com/peerdns/peerdns/pkg/storage"
)

// Manager manages multiple DIDs.
type Manager struct {
	store    *DIDStore
	didCache map[string]*DID
	mu       sync.RWMutex
}

// NewManager initializes a new Manager.
func NewManager(store *storage.Db) *Manager {
	didStore := NewDIDStore(store)
	return &Manager{
		store:    didStore,
		didCache: make(map[string]*DID),
	}
}

// CreateNewDID creates and registers a new DID.
func (im *Manager) CreateNewDID() (*DID, error) {
	did, err := im.store.CreateNewDID()
	if err != nil {
		return nil, fmt.Errorf("failed to create new DID: %w", err)
	}

	im.mu.Lock()
	im.didCache[did.ID] = did
	im.mu.Unlock()

	return did, nil
}

// GetDID retrieves a DID by its ID, utilizing a cache for faster access.
func (im *Manager) GetDID(didID string) (*DID, error) {
	im.mu.RLock()
	did, exists := im.didCache[didID]
	im.mu.RUnlock()
	if exists {
		return did, nil
	}

	// Retrieve from storage if not in cache
	did, err := im.store.GetDID(didID)
	if err != nil {
		return nil, err
	}

	// Cache the retrieved DID
	im.mu.Lock()
	im.didCache[didID] = did
	im.mu.Unlock()

	return did, nil
}

// ListAllDIDs returns a list of all DIDs managed by the Manager.
func (im *Manager) ListAllDIDs() ([]*DID, error) {
	didIDs, err := im.store.storage.ListKeysWithPrefix("")
	if err != nil {
		return nil, fmt.Errorf("failed to list DIDs: %w", err)
	}

	var dids []*DID
	for _, id := range didIDs {
		did, err := im.GetDID(id)
		if err != nil {
			continue // Skip invalid or corrupted DIDs
		}
		dids = append(dids, did)
	}

	return dids, nil
}
