package identity

import (
	"context"
	"os"
	"testing"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func TestManager(t *testing.T) {
	// Set up in-memory storage for testing
	ctx := context.Background()
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "identity_test",
				Path:            "/tmp/test_identity.mdbx",
				MaxReaders:      4096,
				MaxSize:         1,    // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
		},
	}
	storageManager, err := storage.NewManager(ctx, mdbxConfig)
	assert.NoError(t, err)
	defer func() {
		storageManager.Close()
		os.RemoveAll("./data/test_identity.mdbx")
	}()

	identityDb, err := storageManager.GetDb("identity_test")
	assert.NoError(t, err)

	identityManager := NewManager(identityDb.(*storage.Db))

	tests := []struct {
		name          string
		operation     string
		expectedError bool
	}{
		{"Create DID", "create", false},
		{"Get Existing DID", "get_existing", false},
		{"Get Non-Existing DID", "get_non_existing", true},
		{"List All DIDs", "list", false},
	}

	var createdDID *DID

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.operation {
			case "create":
				did, err := identityManager.CreateNewDID()
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, did)
					createdDID = did
				}
			case "get_existing":
				did, err := identityManager.GetDID(createdDID.ID)
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, createdDID.ID, did.ID)
				}
			case "get_non_existing":
				_, err := identityManager.GetDID("non_existing_id")
				if tt.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				/*			case "list":
							dids, err := identityManager.ListAllDIDs()
							if tt.expectedError {
								assert.Error(t, err)
							} else {
								assert.NoError(t, err)
								assert.GreaterOrEqual(t, len(dids), 1)
							}*/
			}
		})
	}
}
