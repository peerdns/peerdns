// pkg/identity/did.go
package identity

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/herumi/bls-go-binary/bls"
	"github.com/peerdns/peerdns/pkg/storage"
)

// DID represents a Decentralized Identifier with associated cryptographic keys.
type DID struct {
	ID         string
	PrivateKey *bls.SecretKey
	PublicKey  *bls.PublicKey
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

// CreateNewDID generates a new DID with a specified initial stake.
func (ds *DIDStore) CreateNewDID(initialStake int) (*DID, error) {
	// Initialize BLS if not already done
	err := bls.Init(bls.BLS12_381)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize BLS: %w", err)
	}

	// Generate a new BLS secret key
	var sk bls.SecretKey
	sk.SetByCSPRNG()
	pk := sk.GetPublicKey()

	// Generate a random DID ID
	didBytes := make([]byte, 16)
	_, err = rand.Read(didBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DID ID: %w", err)
	}
	didID := hex.EncodeToString(didBytes)

	did := &DID{
		ID:         didID,
		PrivateKey: &sk,
		PublicKey:  pk,
	}

	// Persist the DID to storage
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Serialize the DID for storage
	didData := fmt.Sprintf("%s:%s", hex.EncodeToString(did.PrivateKey.Serialize()), hex.EncodeToString(did.PublicKey.Serialize()))
	err = ds.storage.Set([]byte(did.ID), []byte(didData))
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

	// Deserialize the DID
	parts := bytes.Split(data, []byte(":"))
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid DID data format")
	}

	var sk bls.SecretKey
	err = sk.Deserialize(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize secret key: %w", err)
	}

	var pk bls.PublicKey
	err = pk.Deserialize(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize public key: %w", err)
	}

	did := &DID{
		ID:         didID,
		PrivateKey: &sk,
		PublicKey:  &pk,
	}

	return did, nil
}
