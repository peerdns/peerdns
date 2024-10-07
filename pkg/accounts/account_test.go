package accounts

import (
	"fmt"
	"github.com/peerdns/peerdns/pkg/signatures"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorePersistence(t *testing.T) {
	tests := []struct {
		name         string
		persist      bool
		accountName  string
		comment      string
		expectOnDisk bool
	}{
		{
			name:         "Store With Persistence",
			persist:      true,
			accountName:  "Persistent Account",
			comment:      "This is a persistent account.",
			expectOnDisk: true,
		},
		{
			name:         "Store Without Persistence",
			persist:      false,
			accountName:  "Non-Persistent Account",
			comment:      "This account should not be saved on disk.",
			expectOnDisk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			cfg, log := setupTestEnvironment(t)

			// Initialize the Store
			store, err := NewStore(cfg, log)
			require.NoError(t, err, "Failed to create store")

			// Create a sample Account with the specified persistence
			account, err := store.Create(tt.accountName, tt.comment, tt.persist)
			require.NoError(t, err)
			require.NotNil(t, account)

			// Check if the Account is stored in memory and on disk
			assert.Equal(t, tt.accountName, account.Name)
			filePath := cfg.BasePath + "/" + account.ID + ".yaml"
			_, err = os.Stat(filePath)
			if tt.expectOnDisk {
				assert.NoError(t, err, "Account YAML file should be present on disk")
			} else {
				assert.True(t, os.IsNotExist(err), "Account YAML file should not be present on disk")
			}

			// Get the Account by its peerID
			retrievedAccount, err := store.GetByPeerID(account.PeerID)
			assert.NoError(t, err)
			assert.NotNil(t, retrievedAccount)
			assert.Equal(t, account.ID, retrievedAccount.ID)

			// Delete the Account and check removal from both memory and disk
			err = store.Delete(account.PeerID)
			assert.NoError(t, err)
			_, err = store.GetByPeerID(account.PeerID)
			assert.Error(t, err, "Account should be removed from memory")
			_, err = os.Stat(filePath)
			assert.True(t, os.IsNotExist(err), "Account YAML file should be removed from disk")
		})
	}
}

func TestAccountSign(t *testing.T) {
	// Set up test environment
	cfg, log := setupTestEnvironment(t)

	// Initialize the Store
	store, err := NewStore(cfg, log)
	require.NoError(t, err, "Failed to create store")

	// Generate a non-persistent Account using the store
	account, err := store.Create("Test Account", "Testing sign functionality.", false)
	require.NoError(t, err)
	require.NotNil(t, account)

	tests := []struct {
		name        string
		data        []byte
		signer      *Account
		expectError bool
	}{
		{
			name:        "Sign with Valid Signer",
			data:        []byte("Hello, PeerDNS!"),
			signer:      account,
			expectError: false,
		},
		{
			name:        "Sign with Nil Signer",
			data:        []byte("Hello, PeerDNS!"),
			signer:      nil,
			expectError: true,
		},
		{
			name:        "Sign with Empty Data",
			data:        []byte(""),
			signer:      account,
			expectError: true,
		},
		{
			name:        "Sign with Nil Data",
			data:        nil,
			signer:      account,
			expectError: true,
		},
	}

	// Use a specific signer type for testing, e.g., Ed25519
	signerType := signatures.Ed25519SignerType

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var signature []byte
			var err error
			if tt.signer != nil {
				signature, err = tt.signer.Sign(signerType, tt.data)
			} else {
				err = fmt.Errorf("signer is nil")
			}

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, signature)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, signature)
			}
		})
	}
}

func TestAccountVerify(t *testing.T) {
	// Set up test environment
	cfg, log := setupTestEnvironment(t)

	// Initialize the Store
	store, err := NewStore(cfg, log)
	require.NoError(t, err, "Failed to create store")

	// Generate a non-persistent Account using the store
	account, err := store.Create("Test Account", "Testing verify functionality.", false)
	require.NoError(t, err)
	require.NotNil(t, account)

	data := []byte("Hello, PeerDNS!")
	signerType := signatures.Ed25519SignerType
	validSignature, err := account.Sign(signerType, data)
	require.NoError(t, err)
	require.NotNil(t, validSignature)

	otherData := []byte("Other Data")
	otherSignature, err := account.Sign(signerType, otherData)
	require.NoError(t, err)
	require.NotNil(t, otherSignature)

	tests := []struct {
		name        string
		data        []byte
		signature   []byte
		account     *Account
		expectValid bool
		expectError bool
	}{
		{
			name:        "Verify Valid Signature",
			data:        data,
			signature:   validSignature,
			account:     account,
			expectValid: true,
			expectError: false,
		},
		{
			name:        "Verify Invalid Signature",
			data:        data,
			signature:   otherSignature,
			account:     account,
			expectValid: false,
			expectError: false,
		},
		{
			name:        "Verify with Nil Signature",
			data:        data,
			signature:   nil,
			account:     account,
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Verify with Nil Data",
			data:        nil,
			signature:   validSignature,
			account:     account,
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Verify with Empty Data",
			data:        []byte(""),
			signature:   validSignature,
			account:     account,
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Verify with Nil Account",
			data:        data,
			signature:   validSignature,
			account:     nil,
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Verify with Account Missing Signer",
			data:        data,
			signature:   validSignature,
			account:     &Account{}, // Account with empty Signers map
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var valid bool
			var err error
			if tt.account != nil {
				valid, err = tt.account.Verify(signerType, tt.data, tt.signature)
			} else {
				err = fmt.Errorf("Account is nil")
			}

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, valid)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectValid, valid)
			}
		})
	}
}
