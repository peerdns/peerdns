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
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/node"
	"github.com/peerdns/peerdns/pkg/shutdown"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func main() {
	// Example logger configuration
	logConfig := config.Logger{
		Enabled:     true,
		Environment: "development", // or "production"
		Level:       "debug",       // "debug", "info", "warn", "error"
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
	defer shutdownManager.Wait()

	// Use the context from the ShutdownManager
	ctx = shutdownManager.Context()

	// Create an errgroup with the context
	g, ctx := errgroup.WithContext(ctx)

	// Number of validators to initialize
	numValidators := 10
	validators := make([]*node.Node, numValidators)

	// Initialize nodes
	for i := 0; i < numValidators; i++ {
		// Create a unique data directory for the node
		dataDir := fmt.Sprintf("./tmpdata/node%d", i)

		// Create the data directory if it doesn't exist
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			appLogger.Fatal("Failed to create data directory", zap.Error(err), zap.String("dataDir", dataDir))
		}

		// Configure MDBX paths for this node
		mdbxNodes := []config.MdbxNode{
			{
				Name:            "identity",
				Path:            dataDir + "/identity.mdbx",
				MaxReaders:      4096,
				MaxSize:         10,   // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
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
				BootstrapPeers: []string{}, // Add bootstrap peers if any
			},
			Sharding: config.Sharding{
				ShardCount: 4,
			},
		}

		// Initialize the node
		nodeInstance, err := node.NewNode(ctx, nodeConfig, appLogger)
		if err != nil {
			appLogger.Fatal("Failed to initialize node", zap.Error(err), zap.Int("nodeID", i))
		}

		// Store the node
		validators[i] = nodeInstance

		// Register the node's Shutdown method with the ShutdownManager
		shutdownManager.AddShutdownCallback(func() {
			nodeInstance.Shutdown()
		})
	}

	appLogger.Info("Nodes are prepared...")

	// Start nodes in separate goroutines
	for _, nodeInstance := range validators {
		n := nodeInstance

		g.Go(func() error {
			n.Start()
			return nil
		})
	}

	appLogger.Info("Started all nodes....")

	// Collect peer addresses
	peerAddresses := make([]string, numValidators)

	// Allow some time for hosts to start and have their addresses available
	time.Sleep(2 * time.Second)

	// After starting all nodes, collect their addresses
	for i, nodeInstance := range validators {
		// Get the peer addresses
		hostAddrs := nodeInstance.Network.Host.Addrs()
		if len(hostAddrs) == 0 {
			appLogger.Error("No host addresses found", zap.Int("nodeID", i))
			continue
		}
		peerAddr := fmt.Sprintf("%s/p2p/%s", hostAddrs[0].String(), nodeInstance.Network.Host.ID().String())
		peerAddresses[i] = peerAddr
		appLogger.Info("Node started", zap.Int("nodeID", i), zap.String("peerAddr", peerAddr))
	}

	// Connect Validators in a Mesh Network
	for i := 0; i < numValidators; i++ {
		for j := i + 1; j < numValidators; j++ {
			peerAddr := peerAddresses[j]
			err := validators[i].Network.ConnectPeer(peerAddr)
			if err != nil {
				appLogger.Error("Failed to connect peers", zap.Int("fromNode", i), zap.Int("toNode", j), zap.Error(err))
			} else {
				appLogger.Info("Connected peers", zap.Int("fromNode", i), zap.Int("toNode", j))
			}
		}
	}

	// Start Message Broadcasting and Verification Loop
	for i, nodeInstance := range validators {
		i := i // capture loop variable
		n := nodeInstance

		g.Go(func() error {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					// Create a message
					messageContent := []byte(fmt.Sprintf("Block from Validator %d at %s", i, time.Now().Format(time.RFC3339)))

					// Propose a block
					err := n.Consensus.ProposeBlock(ctx, messageContent)
					if err != nil {
						if ctx.Err() != nil {
							// Context canceled, exit
							return ctx.Err()
						}
						appLogger.Error("Failed to propose block", zap.Int("nodeID", i), zap.Error(err))
						continue
					}
					appLogger.Info("Node proposed a block", zap.Int("nodeID", i))
				}
			}
		})
	}

	// Wait for all goroutines to finish or receive an interrupt signal
	go func() {
		<-signalChan
		appLogger.Info("Received interrupt signal, initiating shutdown...")
		shutdownManager.Wait()
	}()

	// Wait for all goroutines to finish
	if err := g.Wait(); err != nil && err != context.Canceled {
		appLogger.Error("Error occurred", zap.Error(err))
	}

	// Application shutdown is handled via the ShutdownManager
	appLogger.Info("Application shut down gracefully.")
}
