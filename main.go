package main

import (
	"context"
	"fmt"
	"log"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/storage"
	"go.uber.org/zap"
)

func main() {
	// Example logger configuration
	logConfig := config.Logger{
		Enabled:     true,
		Environment: "development", // or "production"
		Level:       "debug",       // "debug", "info", "warn", "error"
	}

	if err := logger.InitializeGlobalLogger(logConfig); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Retrieve the global logger
	appLogger := logger.G()

	appLogger.Info("Starting application", zap.String("environment", logConfig.Environment))

	// Initialize storage manager
	ctx := context.Background()
	mdbxConfig := config.Mdbx{
		Enabled: true,
		Nodes: []config.MdbxNode{
			{
				Name:            "identity",
				Path:            "/tmp/identity.mdbx",
				MaxReaders:      4096,
				MaxSize:         1,    // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
		},
	}
	storageManager, err := storage.NewManager(ctx, mdbxConfig)
	if err != nil {
		appLogger.Fatal("Failed to create storage manager", zap.Error(err))
	}
	defer storageManager.Close()

	// Get the identity database
	identityDb, err := storageManager.GetDb("identity")
	if err != nil {
		appLogger.Fatal("Failed to get identity database", zap.Error(err))
	}

	// Initialize the identity manager
	identityManager := identity.NewManager(identityDb.(*storage.Db))

	// Create 10 random identities
	for i := 0; i < 10; i++ {
		did, err := identityManager.CreateNewDID()
		if err != nil {
			appLogger.Error("Failed to create new DID", zap.Error(err))
			continue
		}
		appLogger.Info("Created new DID", zap.String("DID", did.ID))
	}

	// List all DIDs
	dids, err := identityManager.ListAllDIDs()
	if err != nil {
		appLogger.Error("Failed to list DIDs", zap.Error(err))
	} else {
		for _, did := range dids {
			fmt.Printf("DID ID: %s\n", did.ID)
		}
	}

	appLogger.Info("Application finished")
}
