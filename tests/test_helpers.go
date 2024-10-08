package tests

import (
	"context"
	"fmt"
	"github.com/peerdns/peerdns/pkg/accounts"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/node"
	"github.com/peerdns/peerdns/pkg/storage"
)

// TestNode represents a test node with its configuration and instance.
type TestNode struct {
	node   *node.Node
	config config.Config
	peerID peer.ID
	dir    string
	did    *accounts.Account
}

// InitializeTestNodes initializes 'count' number of nodes for testing.
// Each node is assigned a unique port starting from 'basePort'.
// DIDs are created with persistence disabled (non-persistent keys).
func InitializeTestNodes(t *testing.T, ctx context.Context, count int, basePort int) ([]*TestNode, error) {
	var nodes []*TestNode
	nodeAddrs := make([]string, 0, count) // To store the multiaddresses of each node

	for i := 0; i < count; i++ {
		// Create a unique temporary directory for each node
		dir, err := os.MkdirTemp("", fmt.Sprintf("peerdns_node_%d", i))
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory for node %d: %w", i, err)
		}

		// Configure MDBX nodes
		mdbxNodes := []config.MdbxNode{
			{
				Name:            "chain",
				Path:            filepath.Join(dir, "chain.mdbx"),
				MaxReaders:      4096,
				MaxSize:         10,   // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
			{
				Name:            "consensus",
				Path:            filepath.Join(dir, "consensus.mdbx"),
				MaxReaders:      4096,
				MaxSize:         10,
				MinSize:         1,
				GrowthStep:      4096,
				FilePermissions: 0600,
			},
		}

		// Assign a unique port for the node
		listenPort := basePort + i

		// Determine bootstrap peers
		var bootstrapPeers []string
		if i > 0 {
			// Use the first node as the bootstrap node
			bootstrapPeerID := nodes[0].peerID
			bootstrapAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", basePort, bootstrapPeerID.String())
			bootstrapPeers = append(bootstrapPeers, bootstrapAddr)
		}

		// Build the node configuration
		nodeConfig := config.Config{
			Logger: config.Logger{
				Enabled:     true,
				Environment: "development",
				Level:       "debug",
			},
			Mdbx: config.Mdbx{
				Enabled: true,
				Nodes:   mdbxNodes,
			},
			Networking: config.Networking{
				ListenAddrs:    []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)},
				ProtocolID:     "/peerdns/1.0.0",
				BootstrapPeers: bootstrapPeers,
				BootstrapNode:  i == 0, // First node is the bootstrap node
				EnableMDNS:     true,
				EnableRelay:    true,
				InterfaceName:  "lo", // Using loopback interface for testing
			},
			Identity: config.Identity{
				Enabled:  true,
				BasePath: dir,
				Keys:     []config.Key{}, // Start with empty keys; we'll generate them dynamically
			},
			Sharding: config.Sharding{
				ShardCount: 4,
			},
			Observability: config.Observability{
				Metrics: config.MetricsConfig{
					Enable:         false,
					Exporter:       "prometheus",
					Endpoint:       "0.0.0.0:9090",
					Headers:        map[string]string{},
					ExportInterval: 15 * time.Second,
					SampleRate:     1.0,
				},
				Tracing: config.TracingConfig{
					Enable:         false,
					Exporter:       "otlp",
					Endpoint:       "localhost:4317",
					Headers:        map[string]string{},
					Sampler:        "always_on",
					SamplingRate:   1.0,
					ExportInterval: 15 * time.Second,
				},
			},
		}

		// Initialize the logger
		testLogger, err := logger.InitializeGlobalLogger(nodeConfig.Logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize logger for node %d: %w", i, err)
		}

		// Initialize the storage
		storageManager, err := storage.NewManager(ctx, nodeConfig.Mdbx)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize storage for node %d: %w", i, err)
		}

		// Initialize the identity manager
		identityMgr, err := accounts.NewManager(&nodeConfig.Identity, testLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize identity manager for node %d: %w", i, err)
		}

		// Create a DID with persistence disabled (non-persistent keys)
		did, err := identityMgr.Create("Test DID", fmt.Sprintf("This is test DID for node %d", i), false)
		if err != nil {
			return nil, fmt.Errorf("failed to create DID for node %d: %w", i, err)
		}

		// Assign the newly created DID as a peer id in the networking configuration
		nodeConfig.Networking.PeerID = did.PeerID

		// Initialize Observability
		obs, err := observability.New(ctx, &nodeConfig, testLogger)
		if err != nil {
			return nil, fmt.Errorf("failure to initialize observability system for node %d: %w", i, err)
		}

		// Initialize the node with the DID's private key
		instance, err := node.NewNode(ctx, &nodeConfig, testLogger, storageManager, identityMgr, obs)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize node %d: %w", i, err)
		}

		// Start the node
		go func() {
			err := instance.Start()
			require.NoError(t, err)
		}()

		// Wait for the node to start
		stateErr := instance.StateManager().WaitForState(node.NodeStateType, node.Started, 15*time.Second)
		require.NoError(t, stateErr)

		// Append to the nodes slice with the DID
		testNode := &TestNode{
			node:   instance,
			config: nodeConfig,
			peerID: did.PeerID,
			dir:    dir,
			did:    did, // Assign the DID to the TestNode
		}
		nodes = append(nodes, testNode)

		// Construct the multiaddress of this node and store it for others to use
		nodeAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", listenPort, did.PeerID.String())
		nodeAddrs = append(nodeAddrs, nodeAddr)

		// Optionally, log the node address
		testLogger.Info("Initialized node", "index", i, "address", nodeAddr)
	}

	return nodes, nil
}

// ShutdownTestNodes gracefully shuts down all test nodes and cleans up temporary directories.
func ShutdownTestNodes(t *testing.T, nodes []*TestNode) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(nodes))

	for _, nodeInst := range nodes {
		wg.Add(1)
		go func(n *TestNode) {
			defer wg.Done()
			if err := n.node.Shutdown(); err != nil {
				errChan <- fmt.Errorf("failed to shutdown node %s: %w", n.peerID, err)
				return
			}

			// Remove temporary directory
			if err := os.RemoveAll(n.dir); err != nil {
				errChan <- fmt.Errorf("failed to remove temp directory %s: %w", n.dir, err)
				return
			}
		}(nodeInst)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}
