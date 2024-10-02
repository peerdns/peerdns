package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNodeDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	basePort := 4350
	numNodes := 4

	// Initialize four test nodes
	nodes, err := InitializeTestNodes(t, ctx, numNodes, basePort)
	require.NoError(t, err, "Failed to initialize test nodes")

	// Ensure all nodes are shut down and directories are cleaned up after the test
	defer func() {
		err := ShutdownTestNodes(t, nodes)
		require.NoError(t, err, "Failed to shutdown test nodes")
	}()

	// Wait a bit to ensure all nodes are fully started and connected
	time.Sleep(10 * time.Second)
}
