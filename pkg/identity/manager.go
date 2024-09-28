// pkg/identity/identity_manager.go
package identity

import (
	"fmt"
	"sync"

	"github.com/peerdns/peerdns/pkg/storage"
)

// IdentityManager manages multiple DIDs.
type IdentityManager struct {
	store    *DIDStore
	didCache map[string]*DID
	mu       sync.RWMutex
}

// NewIdentityManager initializes a new IdentityManager.
func NewIdentityManager(store *storage.Db) *IdentityManager {
	didStore := NewDIDStore(store)
	return &IdentityManager{
		store:    didStore,
		didCache: make(map[string]*DID),
	}
}

// CreateNewDID creates and registers a new DID with the specified initial stake.
func (im *IdentityManager) CreateNewDID(initialStake int) (*DID, error) {
	did, err := im.store.CreateNewDID(initialStake)
	if err != nil {
		return nil, fmt.Errorf("failed to create new DID: %w", err)
	}

	im.mu.Lock()
	im.didCache[did.ID] = did
	im.mu.Unlock()

	return did, nil
}

// GetDID retrieves a DID by its ID, utilizing a cache for faster access.
func (im *IdentityManager) GetDID(didID string) (*DID, error) {
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

// ListAllDIDs returns a list of all DIDs managed by the IdentityManager.
func (im *IdentityManager) ListAllDIDs() ([]*DID, error) {
	// Assuming the storage package provides a method to list all keys
	didIDs, err := im.store.storage.ListKeys()
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
