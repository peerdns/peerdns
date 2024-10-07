package accounts

import (
	"fmt"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerOperations(t *testing.T) {
	cfg, log := setupTestEnvironment(t)

	// Initialize the Manager with the new YAML-based store
	manager, err := NewManager(&cfg, log)
	require.NoError(t, err, "Failed to create identity manager")

	var createdAccount *Account

	tests := []struct {
		name          string
		operation     func() error
		expectedError bool
	}{
		{
			name: "Create Account",
			operation: func() error {
				var err error
				createdAccount, err = manager.Create("Test Account", "This is a test account.", true)
				return err
			},
			expectedError: false,
		},
		{
			name: "Get Existing Account",
			operation: func() error {
				_, err := manager.GetByPeerID(createdAccount.PeerID)
				return err
			},
			expectedError: false,
		},
		{
			name: "Get Non-Existing Account",
			operation: func() error {
				// Generate a random peer ID that is not in the manager
				privKey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
				if err != nil {
					return err
				}
				pubKey := privKey.GetPublic()
				nonExistentID, err := peer.IDFromPublicKey(pubKey)
				if err != nil {
					return err
				}
				// Ensure the generated ID is different from the created account's ID
				if nonExistentID == createdAccount.PeerID {
					return fmt.Errorf("generated peer ID matches existing account")
				}
				_, err = manager.GetByPeerID(nonExistentID)
				return err
			},
			expectedError: true,
		},
		{
			name: "List All Accounts",
			operation: func() error {
				accounts, err := manager.List()
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
				return manager.Delete(createdAccount.PeerID)
			},
			expectedError: false,
		},
		{
			name: "Delete Non-Existing Account",
			operation: func() error {
				// Attempt to delete the same account again
				err := manager.Delete(createdAccount.PeerID)
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
