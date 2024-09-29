// pkg/identity/did.go
package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/peerdns/peerdns/pkg/encryption"
	"github.com/peerdns/peerdns/pkg/storage"
)

// DID represents a Decentralized Identifier with associated cryptographic keys.
type DID struct {
	ID         string
	PrivateKey *encryption.BLSPrivateKey
	PublicKey  *encryption.BLSPublicKey
}

// DIDStore manages the persistence of DIDs.
type DIDStore struct {
	storage *storage.Db
	mu      sync.RWMutex
}

// NewDIDStore initializes a new DIDStore.
func NewDIDStore(store *storage.Db) *DIDStore {
	return &DIDStore{
		storage: store,
	}
}

// CreateNewDID generates a new DID.
func (ds *DIDStore) CreateNewDID() (*DID, error) {
	// Initialize BLS if not already done
	err := encryption.InitBLS()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize BLS: %w", err)
	}

	// Generate a new BLS key pair
	sk, pk, err := encryption.GenerateBLSKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate BLS keys: %w", err)
	}

	// Generate a random DID ID
	didBytes := make([]byte, 16)
	_, err = rand.Read(didBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DID ID: %w", err)
	}
	didID := hex.EncodeToString(didBytes)

	did := &DID{
		ID:         didID,
		PrivateKey: sk,
		PublicKey:  pk,
	}

	// Serialize the DID for storage
	privBytes, err := sk.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize private key: %w", err)
	}
	pubBytes, err := pk.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize public key: %w", err)
	}
	didData := append(privBytes, pubBytes...)

	// Persist the DID to storage
	ds.mu.Lock()
	defer ds.mu.Unlock()
	err = ds.storage.Set([]byte(did.ID), didData)
	if err != nil {
		return nil, fmt.Errorf("failed to store DID: %w", err)
	}

	return did, nil
}

// GetDID retrieves a DID by its ID.
func (ds *DIDStore) GetDID(didID string) (*DID, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	// Retrieve DID data from storage
	data, err := ds.storage.Get([]byte(didID))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve DID: %w", err)
	}

	// Deserialize the private key (32 bytes)
	if len(data) < 32 {
		return nil, fmt.Errorf("invalid DID data format")
	}
	privBytes := data[:32]
	pubBytes := data[32:]

	sk, err := encryption.DeserializePrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize private key: %w", err)
	}

	pk, err := encryption.DeserializePublicKey(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize public key: %w", err)
	}

	did := &DID{
		ID:         didID,
		PrivateKey: sk,
		PublicKey:  pk,
	}

	return did, nil
}
