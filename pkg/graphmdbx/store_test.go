// pkg/graphmdbx/store_test.go

package graphmdbx

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"testing"

	"github.com/dominikbraun/graph"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_AddAndRetrieveVertex(t *testing.T) {
	// Initialize storage provider using the test helper
	sp := storage.NewMDBXProviderForTest(t)
	defer sp.Close()

	// Initialize graphmdbx.Store with both K and T
	vertexPrefix := "vertex"
	edgePrefix := "edge"
	ctx := context.Background()
	store := New[string, string](sp, vertexPrefix, edgePrefix, ctx)

	// Define test data
	hash := "vertex1"
	value := "Test Vertex"
	properties := graph.VertexProperties{
		Weight: 1, // Ensure the type matches (int)
	}

	// Add vertex
	err := store.AddVertex(hash, value, properties)
	require.NoError(t, err, "Failed to add vertex")

	// Retrieve vertex
	retrievedValue, retrievedProps, err := store.Vertex(hash)
	require.NoError(t, err, "Failed to retrieve vertex")
	assert.Equal(t, value, retrievedValue, "Vertex values should match")
	assert.Equal(t, properties.Weight, retrievedProps.Weight, "Vertex weights should match")
}

func TestStore_RemoveVertex(t *testing.T) {
	// Initialize storage provider using the test helper
	sp := storage.NewMDBXProviderForTest(t)
	defer sp.Close()

	// Initialize graphmdbx.Store with both K and T
	vertexPrefix := "vertex"
	edgePrefix := "edge"
	ctx := context.Background()
	store := New[string, string](sp, vertexPrefix, edgePrefix, ctx)

	// Define test data
	hash := "vertex1"
	value := "Test Vertex"
	properties := graph.VertexProperties{
		Weight: 1, // Ensure the type matches (int)
	}

	// Add vertex
	err := store.AddVertex(hash, value, properties)
	require.NoError(t, err, "Failed to add vertex")

	// Remove vertex
	err = store.RemoveVertex(hash)
	require.NoError(t, err, "Failed to remove vertex")

	// Attempt to retrieve vertex
	_, _, err = store.Vertex(hash)
	require.Error(t, err, "Expected an error when retrieving a removed vertex")

	if !errors.Is(err, graph.ErrVertexNotFound) {
		t.Fatalf("Expected key not found error, got: %v", err)
	}
}

func TestStore_AddAndRetrieveEdge(t *testing.T) {
	// Initialize storage provider using the test helper
	sp := storage.NewMDBXProviderForTest(t)
	defer sp.Close()

	// Initialize graphmdbx.Store with both K and T
	vertexPrefix := "vertex"
	edgePrefix := "edge"
	ctx := context.Background()
	store := New[string, string](sp, vertexPrefix, edgePrefix, ctx)

	// Define test data
	source := "vertex1"
	target := "vertex2"
	edgeProperties := graph.EdgeProperties{
		Weight: 1, // Ensure EdgeProperties.Weight is int
	}

	// Add source and target vertices
	err := store.AddVertex(source, "Source Vertex", graph.VertexProperties{})
	require.NoError(t, err, "Failed to add source vertex")

	err = store.AddVertex(target, "Target Vertex", graph.VertexProperties{})
	require.NoError(t, err, "Failed to add target vertex")

	// Add edge
	edge := graph.Edge[string]{
		Source:     source,
		Target:     target,
		Properties: edgeProperties,
	}
	err = store.AddEdge(source, target, edge)
	require.NoError(t, err, "Failed to add edge")

	// Retrieve edge
	retrievedEdge, err := store.Edge(source, target)
	require.NoError(t, err, "Failed to retrieve edge")
	assert.Equal(t, edge.Properties.Weight, retrievedEdge.Properties.Weight, "Edge weights should match")
}

func TestStore_UpdateEdge(t *testing.T) {
	// Initialize storage provider using the test helper
	sp := storage.NewMDBXProviderForTest(t)
	defer sp.Close()

	// Initialize graphmdbx.Store with both K and T
	vertexPrefix := "vertex"
	edgePrefix := "edge"
	ctx := context.Background()
	store := New[string, string](sp, vertexPrefix, edgePrefix, ctx)

	// Define test data
	source := "vertex1"
	target := "vertex2"
	initialEdgeProperties := graph.EdgeProperties{
		Weight: 1, // Ensure EdgeProperties.Weight is int
	}
	updatedEdgeProperties := graph.EdgeProperties{
		Weight: 2, // Ensure EdgeProperties.Weight is int
	}

	// Add source and target vertices
	err := store.AddVertex(source, "Source Vertex", graph.VertexProperties{})
	require.NoError(t, err, "Failed to add source vertex")

	err = store.AddVertex(target, "Target Vertex", graph.VertexProperties{})
	require.NoError(t, err, "Failed to add target vertex")

	// Add edge
	edge := graph.Edge[string]{
		Source:     source,
		Target:     target,
		Properties: initialEdgeProperties,
	}
	err = store.AddEdge(source, target, edge)
	require.NoError(t, err, "Failed to add edge")

	// Update edge
	updatedEdge := graph.Edge[string]{
		Source:     source,
		Target:     target,
		Properties: updatedEdgeProperties,
	}
	err = store.UpdateEdge(source, target, updatedEdge)
	require.NoError(t, err, "Failed to update edge")

	// Retrieve edge
	retrievedEdge, err := store.Edge(source, target)
	require.NoError(t, err, "Failed to retrieve edge")
	assert.Equal(t, updatedEdge.Properties.Weight, retrievedEdge.Properties.Weight, "Edge weights should match after update")
}

func TestStore_ListVertices(t *testing.T) {
	// Initialize storage provider using the test helper
	sp := storage.NewMDBXProviderForTest(t)
	defer sp.Close()

	// Initialize graphmdbx.Store with both K and T
	vertexPrefix := "vertex"
	edgePrefix := "edge"
	ctx := context.Background()
	store := New[string, string](sp, vertexPrefix, edgePrefix, ctx)

	// Add multiple vertices
	vertices := map[string]string{
		"vertex1": "First Vertex",
		"vertex2": "Second Vertex",
		"vertex3": "Third Vertex",
	}

	for hash, value := range vertices {
		err := store.AddVertex(hash, value, graph.VertexProperties{})
		require.NoError(t, err, fmt.Sprintf("Failed to add vertex %s", hash))
	}

	// List vertices
	listedVertices, err := store.ListVertices()
	require.NoError(t, err, "Failed to list vertices")

	// Verify all vertices are listed
	expectedVertices := []string{"vertex1", "vertex2", "vertex3"}
	assert.ElementsMatch(t, expectedVertices, listedVertices, "Listed vertices should match added vertices")
}

func TestStore_ListEdges(t *testing.T) {
	// Initialize storage provider using the test helper
	sp := storage.NewMDBXProviderForTest(t)
	defer sp.Close()

	// Initialize graphmdbx.Store with both K and T
	vertexPrefix := "vertex"
	edgePrefix := "edge"
	ctx := context.Background()
	store := New[string, string](sp, vertexPrefix, edgePrefix, ctx)

	// Add multiple vertices
	vertices := []string{"vertex1", "vertex2", "vertex3"}
	for _, hash := range vertices {
		err := store.AddVertex(hash, fmt.Sprintf("Vertex %s", hash), graph.VertexProperties{})
		require.NoError(t, err, fmt.Sprintf("Failed to add vertex %s", hash))
	}

	// Add multiple edges
	edges := []struct {
		source string
		target string
	}{
		{"vertex1", "vertex2"},
		{"vertex2", "vertex3"},
	}

	for _, edgeData := range edges {
		edge := graph.Edge[string]{
			Source:     edgeData.source,
			Target:     edgeData.target,
			Properties: graph.EdgeProperties{Weight: 1},
		}
		err := store.AddEdge(edgeData.source, edgeData.target, edge)
		require.NoError(t, err, fmt.Sprintf("Failed to add edge from %s to %s", edgeData.source, edgeData.target))
	}

	// List edges
	listedEdges, err := store.ListEdges()
	require.NoError(t, err, "Failed to list edges")

	// Define expected edges
	expectedEdges := []graph.Edge[string]{
		{
			Source: "vertex1",
			Target: "vertex2",
			Properties: graph.EdgeProperties{
				Weight: 1,
			},
		},
		{
			Source: "vertex2",
			Target: "vertex3",
			Properties: graph.EdgeProperties{
				Weight: 1,
			},
		},
	}

	// Verify all edges are listed
	assert.ElementsMatch(t, expectedEdges, listedEdges, "Listed edges should match added edges")
}
