package consensus

import (
	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/stretchr/testify/require"
	"testing"
)

// setupTestEnvironment initializes the test environment by creating a temporary directory
// and initializing the logger. It returns the configuration and logger instances.
func setupTestEnvironment(t testing.TB, level string) (logger.Logger, *accounts.Store) {
	// Set up a temporary directory for YAML file-based storage
	tempDir := t.TempDir()
	cfg := config.Identity{
		Enabled:  true,
		BasePath: tempDir,
	}

	gLog, err := logger.InitializeGlobalLogger(config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       level,
	})

	store, err := accounts.NewStore(cfg, gLog)
	require.NoError(t, err, "Failed to create store")

	return gLog, store
}

// printValidators logs all validators in the ValidatorSet.
func printValidators(t testing.TB, validators *ValidatorSet) {
	t.Helper()
	t.Logf("Current Validators:")
	for _, validator := range validators.GetAllValidators() {
		t.Logf("- ID: %s, PublicKey: %v", validator.PeerID(), validator.PublicKey())
	}
}
