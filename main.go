// main.go

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/node"
	"github.com/peerdns/peerdns/pkg/shutdown"
	"go.uber.org/zap"
)

func main() {
	// Example logger configuration
	logConfig := config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       "debug",
	}

	// Initialize the global logger
	if err := logger.InitializeGlobalLogger(logConfig); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Ensure that logs are flushed before exiting
	defer func() {
		logger.SyncGlobalLogger()
	}()

	// Retrieve the global logger
	appLogger := logger.G()

	appLogger.Info("Starting application", zap.String("environment", logConfig.Environment))

	// Create a parent context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS interrupt signals for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	// Create a ShutdownManager
	shutdownManager := shutdown.NewShutdownManager(ctx, appLogger)
	shutdownManager.Start()
	// Note: We will not defer shutdownManager.Wait() here; we'll call it explicitly later.

	// Number of validators to initialize
	numValidators := 5
	dids := make([]*identity.DID, numValidators)

	// Initialize nodes
	nodes := make([]*node.Node, numValidators)
	nodeAddrs := make([]string, numValidators) // To store the multiaddresses of each node

	for i := 0; i < numValidators; i++ {
		// Create a unique data directory for the node
		dataDir := fmt.Sprintf("./tmpdata/node%d", i)

		// Check if the directory already exists
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			// Create the directory with restrictive permissions (0700)
			if mErr := os.MkdirAll(dataDir, 0700); mErr != nil {
				appLogger.Fatal("Failed to create data directory", zap.String("directory", dataDir), zap.Error(mErr))
			}
			appLogger.Info("Created data directory for node", zap.String("directory", dataDir), zap.Int("nodeIndex", i))
		}

		// Configure MDBX paths for this node
		mdbxNodes := []config.MdbxNode{
			{
				Name:            "chain",
				Path:            dataDir + "/chain.mdbx",
				MaxReaders:      4096,
				MaxSize:         10,   // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
			{
				Name:            "consensus",
				Path:            dataDir + "/consensus.mdbx",
				MaxReaders:      4096,
				MaxSize:         10,   // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
		}

		// Assign a unique port for the node, e.g., starting from 4350
		listenPort := 4350 + i

		// For the first node, BootstrapPeers will be empty
		var bootstrapPeers []string

		if i > 0 {
			// For nodes after the first, include the addresses of previous nodes
			bootstrapPeers = append(bootstrapPeers, nodeAddrs[0]) // Always include Node 0 as bootstrap peer
			// Optionally, include all previous nodes as bootstrap peers
			// for j := 0; j < i; j++ {
			// 	bootstrapPeers = append(bootstrapPeers, nodeAddrs[j])
			// }
		}

		// Build the node configuration
		nodeConfig := config.Config{
			Logger: logConfig,
			Mdbx: config.Mdbx{
				Enabled: true,
				Nodes:   mdbxNodes,
			},
			Networking: config.Networking{
				ListenPort:     listenPort,
				ProtocolID:     "/peerdns/1.0.0",
				BootstrapPeers: bootstrapPeers,
				EnableMDNS:     true, // Enable mDNS
			},
			Sharding: config.Sharding{
				ShardCount: 4,
			},
			Identity: config.Identity{
				Enabled:  true,
				BasePath: dataDir,
			},
		}

		nodeInstance, err := node.NewNode(ctx, nodeConfig, appLogger)
		if err != nil {
			appLogger.Fatal("Failed to initialize node", zap.Error(err), zap.Int("nodeID", i))
		}

		// Store the node
		nodes[i] = nodeInstance

		// Register the node's Shutdown method with the ShutdownManager
		shutdownManager.AddShutdownCallback(func() {
			if sErr := nodeInstance.Shutdown(); sErr != nil {
				appLogger.Fatal("Failed to shutdown node", zap.Error(sErr))
			}
		})

		// Retrieve or create a DID for this node using the identity manager
		did, err := nodeInstance.IdentityManager.Create("Validator Identity", fmt.Sprintf("Node %d Identity", i), true)
		if err != nil {
			appLogger.Fatal("Failed to create or load DID", zap.Error(err), zap.Int("nodeID", i))
		}

		// Collect DID for each node
		dids[i] = did

		// Log validator DID info
		appLogger.Info("Validator DID", zap.Int("validatorIndex", i),
			zap.String("peerID", did.PeerID.String()),
			zap.String("DID", did.ID))

		// Start the node
		go func(n *node.Node) {
			n.Start()
		}(nodeInstance)

		// Construct the multiaddress of this node and store it for others to use
		nodeAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", listenPort, nodeInstance.Network.Host.ID().String())
		nodeAddrs[i] = nodeAddr

		// Wait a bit for the node to start
		time.Sleep(1 * time.Second)
	}

	appLogger.Info("Started all nodes....")

	// Start a goroutine to display metrics periodically
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for i, nodeInstance := range nodes {
					metricsMap := nodeInstance.MetricsCollector.GetAllMetrics()
					if len(metricsMap) == 0 {
						appLogger.Warn("No metrics available for any peers", zap.Int("nodeID", i))
						continue
					}
					for peerID, metrics := range metricsMap {
						utilityScore := nodeInstance.MetricsCollector.CalculateUtilityScore(peerID)
						appLogger.Info("Peer metrics",
							zap.Int("nodeID", i),
							zap.String("peerID", peerID.String()),
							zap.Float64("BandwidthUsage", metrics.BandwidthUsage),
							zap.Float64("Computational", metrics.Computational),
							zap.Float64("Storage", metrics.Storage),
							zap.Float64("Uptime", metrics.Uptime),
							zap.Float64("Responsiveness", metrics.Responsiveness),
							zap.Float64("Reliability", metrics.Reliability),
							zap.Float64("UtilityScore", utilityScore),
						)
					}
				}
			}
		}
	}()

	// Wait for interrupt signal
	<-signalChan
	appLogger.Info("Received interrupt signal, initiating shutdown...")

	// Cancel the context to signal all goroutines to stop
	cancel()

	// Wait for shutdown to complete
	shutdownManager.Wait()

	appLogger.Info("Shutdown complete. Exiting.")
}
