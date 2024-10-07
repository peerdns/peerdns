package accounts

import (
	"crypto/rand"
	"fmt"
	"github.com/peerdns/peerdns/pkg/types"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreOperations(t *testing.T) {
	cfg, log := setupTestEnvironment(t)

	// Initialize the Store
	store, err := NewStore(cfg, log)
	require.NoError(t, err, "Failed to create store")

	var createdAccount *Account
	var peerID peer.ID

	tests := []struct {
		name          string
		operation     func() error
		expectedError bool
	}{
		{
			name: "Create Account",
			operation: func() error {
				var err error
				createdAccount, err = store.Create("Test Account", "This is a test account.", true)
				if err == nil {
					peerID = createdAccount.PeerID
				}
				return err
			},
			expectedError: false,
		},
		{
			name: "Load All Accounts",
			operation: func() error {
				return store.Load()
			},
			expectedError: false,
		},
		{
			name: "Get Existing Account by PeerID",
			operation: func() error {
				_, err := store.GetByPeerID(peerID)
				return err
			},
			expectedError: false,
		},
		{
			name: "Get Non-Existing Account by PeerID",
			operation: func() error {
				// Generate a valid but non-existent peer ID
				privKey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
				if err != nil {
					return err
				}
				pubKey := privKey.GetPublic()
				nonExistentID, err := peer.IDFromPublicKey(pubKey)
				if err != nil {
					return err
				}
				_, err = store.GetByPeerID(nonExistentID)
				return err
			},
			expectedError: true,
		},
		{
			name: "Get Existing Account by Address",
			operation: func() error {
				_, err := store.GetByAddress(createdAccount.Address)
				return err
			},
			expectedError: false,
		},
		{
			name: "Get Non-Existing Account by Address",
			operation: func() error {
				// Generate a random address
				randomAddressBytes := make([]byte, 20)
				_, err := rand.Read(randomAddressBytes)
				if err != nil {
					return err
				}
				randomAddress := types.FromBytes(randomAddressBytes)
				_, err = store.GetByAddress(randomAddress)
				return err
			},
			expectedError: true,
		},
		{
			name: "List All Accounts",
			operation: func() error {
				accounts, err := store.List()
				if err != nil {
					return err
				}
				if len(accounts) == 0 {
					return fmt.Errorf("expected at least one account")
				}
				return nil
			},
			expectedError: false,
		},
		{
			name: "Delete Existing Account",
			operation: func() error {
				return store.Delete(peerID)
			},
			expectedError: false,
		},
		{
			name: "Delete Non-Existing Account",
			operation: func() error {
				// Attempt to delete the same account again
				err := store.Delete(peerID)
				return err
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if tt.expectedError {
				assert.Error(t, err, "Expected error in operation: %s", tt.name)
			} else {
				assert.NoError(t, err, "Unexpected error in operation: %s", tt.name)
			}
		})
	}

	// Clean up the temporary directory after tests
	os.RemoveAll(cfg.BasePath)
}
