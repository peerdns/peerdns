package identity

import (
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestStoreOperations(t *testing.T) {
	// Set up a temporary directory for YAML file-based storage
	tempDir := t.TempDir()
	cfg := config.Identity{
		Enabled:  true,
		BasePath: tempDir,
	}

	// Initialize the logger
	err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	})
	require.NoError(t, err, "Failed to initialize logger")

	// Initialize the Store
	store, err := NewStore(cfg, logger.G())
	require.NoError(t, err, "Failed to create store")

	// Create a sample peer ID for testing
	createdDID, err := store.Create("Test DID", "This is a test DID.", true)
	assert.NoError(t, err)
	assert.NotNil(t, createdDID)

	// Capture the created peerID dynamically
	peerID := createdDID.PeerID

	// Test Load (verify that it loads the created DID from the file)
	t.Run("Load All DIDs", func(t *testing.T) {
		err := store.Load()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(store.keys), 1)
	})

	// Test Get (use the dynamic peerID instead of a hardcoded one)
	t.Run("Get Existing DID", func(t *testing.T) {
		did, err := store.Get(peerID)
		assert.NoError(t, err)
		assert.NotNil(t, did)
		assert.Equal(t, did.PeerID, peerID)
	})

	// Test Get Non-Existing DID (use an invalid peerID for testing)
	t.Run("Get Non-Existing DID", func(t *testing.T) {
		nonExistentID, _ := peer.Decode("12D3KooWLtVhJsdZ4x5NioiGkm7pkmfk3qzZGj4yFXb1UgJInvalid") // Invalid ID for testing
		_, err := store.Get(nonExistentID)
		assert.Error(t, err)
	})

	// Test List (check if the created DID is listed)
	t.Run("List All DIDs", func(t *testing.T) {
		dids, err := store.List()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(dids), 1)
	})

	// Test Delete (delete the dynamically created peerID)
	t.Run("Delete Existing DID", func(t *testing.T) {
		err := store.Delete(peerID)
		assert.NoError(t, err)
	})

	// Test Delete Non-Existing DID (delete an invalid peerID)
	t.Run("Delete Non-Existing DID", func(t *testing.T) {
		nonExistentID, _ := peer.Decode("12D3KooWLtVhJsdZ4x5NioiGkm7pkmfk3qzZGj4yFXb1UgJInvalid") // Invalid ID for testing
		err := store.Delete(nonExistentID)
		assert.Error(t, err)
	})

	// Clean up the temporary directory after tests
	os.RemoveAll(tempDir)
}
