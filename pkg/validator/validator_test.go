// pkg/validator/validator_test.go
package validator

import (
	"testing"

	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/sharding"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewValidator(t *testing.T) {
	// Initialize logger
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, err := loggerConfig.Build()
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Create a mock DID
	did := &identity.DID{
		ID: "test-did",
		// Initialize keys as needed
	}

	// Initialize shard manager
	shardManager := sharding.NewShardManager(4)

	// Create validator
	validatorInstance := NewValidator(did, shardManager, logger)

	if validatorInstance.DID.ID != did.ID {
		t.Errorf("Expected DID ID %s, got %s", did.ID, validatorInstance.DID.ID)
	}

	// Check if validator is assigned to a shard
	shardID, err := shardManager.GetShardForValidator(validatorInstance.ValidatorInfo.ID)
	if err != nil {
		t.Errorf("Failed to get shard for validator: %v", err)
	}

	if shardID < 0 || shardID >= shardManager.ShardCount {
		t.Errorf("Invalid shard ID assigned: %d", shardID)
	}
}
