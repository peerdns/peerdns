package identity

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStoreWithPersistence tests the Store functionality with persistence enabled.
func TestStoreWithPersistence(t *testing.T) {
	// Set up a temporary directory for YAML file-based storage
	tempDir := t.TempDir()
	cfg := config.Identity{
		Enabled:  true,
		BasePath: tempDir,
	}

	// Initialize the logger
	_, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err, "Failed to initialize logger")

	// Initialize the Store
	store, err := NewStore(cfg, logger.G())
	require.NoError(t, err, "Failed to create store")

	// Create a sample DID with persistence enabled
	did, err := store.Create("Persistent DID", "This is a persistent DID.", true)
	require.NoError(t, err)
	require.NotNil(t, did)

	// Check if the DID is stored in memory and on disk
	assert.Equal(t, "Persistent DID", did.Name)
	filePath := tempDir + "/" + did.ID + ".yaml"
	_, err = os.Stat(filePath)
	assert.NoError(t, err, "DID YAML file should be present on disk")

	// Get the DID by its peerID
	retrievedDID, err := store.Get(did.PeerID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedDID)
	assert.Equal(t, did.ID, retrievedDID.ID)

	// Delete the DID and check removal from both memory and disk
	err = store.Delete(did.PeerID)
	assert.NoError(t, err)
	_, err = store.Get(did.PeerID)
	assert.Error(t, err, "DID should be removed from memory")
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "DID YAML file should be removed from disk")
}

// TestStoreWithoutPersistence tests the Store functionality without persistence.
func TestStoreWithoutPersistence(t *testing.T) {
	// Set up a temporary directory for YAML file-based storage
	tempDir := t.TempDir()
	cfg := config.Identity{
		Enabled:  true,
		BasePath: tempDir,
	}

	// Initialize the logger
	_, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err, "Failed to initialize logger")

	// Initialize the Store
	store, err := NewStore(cfg, logger.G())
	require.NoError(t, err, "Failed to create store")

	// Create a sample DID without persistence
	did, err := store.Create("Non-Persistent DID", "This DID should not be saved on disk.", false)
	require.NoError(t, err)
	require.NotNil(t, did)

	// Check if the DID is stored in memory but not on disk
	assert.Equal(t, "Non-Persistent DID", did.Name)
	filePath := tempDir + "/" + did.ID + ".yaml"
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "DID YAML file should not be present on disk")

	// Get the DID by its peerID
	retrievedDID, err := store.Get(did.PeerID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedDID)
	assert.Equal(t, did.ID, retrievedDID.ID)

	// Delete the DID and check removal from memory only
	err = store.Delete(did.PeerID)
	_, err = store.Get(did.PeerID)
	assert.Error(t, err, "DID should be removed from memory")
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "DID YAML file should not exist on disk since persistence is disabled")
}

func TestDIDSignAndVerifyUsingStore(t *testing.T) {
	// Set up a temporary directory for YAML file-based storage (not used in this test)
	tempDir := t.TempDir()
	cfg := config.Identity{
		Enabled:  true,
		BasePath: tempDir,
	}

	// Initialize the logger
	_, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err, "Failed to initialize logger")

	// Initialize the Store
	store, err := NewStore(cfg, logger.G())
	require.NoError(t, err, "Failed to create store")

	// Generate a non-persistent DID using the store
	did, err := store.Create("Non-Persistent DID", "This is a non-persistent DID for testing.", false)
	require.NoError(t, err, "Failed to create non-persistent DID")
	require.NotNil(t, did, "DID should not be nil")

	spew.Dump(did)

	// Test signing functionality
	t.Run("Sign Data Using Store", func(t *testing.T) {
		data := []byte("Hello, PeerDNS!")
		signature, err := did.Sign(data)
		assert.NoError(t, err, "Failed to sign data")
		assert.NotNil(t, signature, "Signature should not be nil")
	})

	// Test verification functionality
	t.Run("Verify Correct Signature Using Store", func(t *testing.T) {
		data := []byte("Hello, PeerDNS!")
		signature, err := did.Sign(data)
		require.NoError(t, err, "Failed to sign data")

		valid, err := did.Verify(data, signature)
		assert.NoError(t, err, "Verification should not return an error")
		assert.True(t, valid, "Signature should be valid for the signed data")
	})

	// Test verification with incorrect data
	t.Run("Verify Incorrect Data Using Store", func(t *testing.T) {
		data := []byte("Hello, PeerDNS!")
		signature, err := did.Sign(data)
		require.NoError(t, err, "Failed to sign data")

		// Modify the data to invalidate the signature
		incorrectData := []byte("Hello, Modified!")
		valid, err := did.Verify(incorrectData, signature)
		assert.NoError(t, err, "Verification should not return an error for incorrect data")
		assert.False(t, valid, "Signature should be invalid for modified data")
	})

	// Test verification with nil signature
	t.Run("Verify Nil Signature Using Store", func(t *testing.T) {
		data := []byte("Hello, PeerDNS!")
		valid, err := did.Verify(data, nil)
		assert.Error(t, err, "Verification should fail with nil signature")
		assert.False(t, valid, "Verification should be false with nil signature")
	})

	// Test signing with nil private key
	t.Run("Sign With Nil Private Key Using Store", func(t *testing.T) {
		// Create a new DID without a private key for signing
		invalidDID := NewDID(did.PeerID, did.PeerPrivateKey, did.PeerPublicKey, nil, did.SigningPublicKey, "Invalid DID", "This DID has no private key")
		data := []byte("Hello, PeerDNS!")
		_, err := invalidDID.Sign(data)
		assert.Error(t, err, "Signing should fail when private key is nil")
	})

	// Test verification with nil public key
	t.Run("Verify With Nil Public Key Using Store", func(t *testing.T) {
		// Create a new DID without a public key for signing
		invalidDID := NewDID(did.PeerID, did.PeerPrivateKey, did.PeerPublicKey, did.SigningPrivateKey, nil, "Invalid DID", "This DID has no public key")
		data := []byte("Hello, PeerDNS!")
		signature, err := did.Sign(data)
		require.NoError(t, err, "Failed to sign data")

		valid, err := invalidDID.Verify(data, signature)
		assert.Error(t, err, "Verification should fail when public key is nil")
		assert.False(t, valid, "Signature verification should not be valid with nil public key")
	})
}
