package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/stretchr/testify/require"
)

// NewMDBXProviderForTest initializes a new MDBXProvider for testing purposes.
// It creates a temporary directory and configures the MDBXNode accordingly.
func NewMDBXProviderForTest(t *testing.T) Provider {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "testdb.mdbx")

	// Define test MDBXNode configuration
	node := config.MdbxNode{
		Name:            "testdb",
		Path:            dbPath,
		MaxReaders:      4096,
		MaxSize:         1,    // 1 GB for testing
		MinSize:         1,    // 1 MB
		GrowthStep:      4096, // 4KB
		FilePermissions: 0600,
	}

	// Initialize the MDBXProvider
	provider, err := NewDb(context.Background(), node)
	require.NoError(t, err, "Failed to initialize MDBXProvider for test")

	return provider
}
