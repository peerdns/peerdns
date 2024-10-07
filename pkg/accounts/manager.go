package accounts

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/types"
	"github.com/pkg/errors"
)

// Manager is a wrapper around Store that provides higher-level operations.
type Manager struct {
	store *Store
}

// NewManager initializes a new Manager with the given configuration and logger.
func NewManager(cfg *config.Identity, logger logger.Logger) (*Manager, error) {
	store, err := NewStore(*cfg, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize store for manager")
	}
	return &Manager{store: store}, nil
}

// Create creates and registers a new DID using the store.
func (m *Manager) Create(name, comment string, presist bool) (*Account, error) {
	did, err := m.store.Create(name, comment, presist)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new DID")
	}
	return did, nil
}

// Load loads all DIDs from the store into memory.
func (m *Manager) Load() error {
	if err := m.store.Load(); err != nil {
		return errors.Wrap(err, "failed to load DIDs in manager")
	}
	return nil
}

// GetByPeerID retrieves an Account by its peer.ID from the store.
func (m *Manager) GetByPeerID(peerID peer.ID) (*Account, error) {
	did, err := m.store.GetByPeerID(peerID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get DID for peer ID %s", peerID.String())
	}
	return did, nil
}

// GetByAddress retrieves an Account by its types.Address from the store.
func (m *Manager) GetByAddress(addr types.Address) (*Account, error) {
	did, err := m.store.GetByAddress(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get DID for peer ID %s", addr.Hex())
	}
	return did, nil
}

// Delete removes a DID by its peer.ID from storage.
func (m *Manager) Delete(peerID peer.ID) error {
	if err := m.store.Delete(peerID); err != nil {
		return errors.Wrapf(err, "failed to delete DID for peer ID %s", peerID.String())
	}
	return nil
}

// List returns a list of all DIDs managed by the store.
func (m *Manager) List() ([]*Account, error) {
	dids, err := m.store.List()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list DIDs in manager")
	}
	return dids, nil
}
