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

func TestManager(t *testing.T) {
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

	// Initialize the Manager with the new YAML-based store
	manager, err := NewManager(&cfg, logger.G())
	require.NoError(t, err, "Failed to create identity manager")

	tests := []struct {
		name          string
		operation     string
		expectedError bool
	}{
		{"Create DID", "create", false},
		{"Get Existing DID", "get_existing", false},
		{"Get Non-Existing DID", "get_non_existing", true},
		{"List All DIDs", "list", false},
		{"Delete Existing DID", "delete_existing", false},
		{"Delete Non-Existing DID", "delete_non_existing", true},
	}

	var createdDID *DID

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.operation {
			case "create":
				// Create a new DID (no peerID parameter required)
				did, err := manager.Create("Test DID", "This is a test DID.", true)
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, did)
					createdDID = did
				}

			case "get_existing":
				// Get the existing DID by its ID
				did, err := manager.Get(createdDID.PeerID)
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, createdDID.ID, did.ID)
				}

			case "get_non_existing":
				// Attempt to get a non-existing DID
				nonExistentID, _ := peer.Decode("12D3KooWLtVhJsdZ4x5NioiGkm7pkmfk3qzZGj4yFXb1UgJInvalid") // Invalid ID for testing
				_, err := manager.Get(nonExistentID)
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

			case "list":
				// List all DIDs
				dids, err := manager.List()
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.GreaterOrEqual(t, len(dids), 1)
				}

			case "delete_existing":
				// Delete the existing DID
				err := manager.Delete(createdDID.PeerID)
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

			case "delete_non_existing":
				// Attempt to delete a non-existing DID
				nonExistentID, _ := peer.Decode("12D3KooWLtVhJsdZ4x5NioiGkm7pkmfk3qzZGj4yFXb1UgJInvalid") // Invalid ID for testing
				err := manager.Delete(nonExistentID)
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}

	// Clean up the temporary directory after tests
	os.RemoveAll(tempDir)
}
