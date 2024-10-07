package accounts

import (
	"testing"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/stretchr/testify/require"
)

// setupTestEnvironment initializes the test environment by creating a temporary directory
// and initializing the logger. It returns the configuration and logger instances.
func setupTestEnvironment(t *testing.T) (config.Identity, logger.Logger) {
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

	return cfg, logger.G()
}
