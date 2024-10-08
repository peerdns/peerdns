package graphmdbx

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"

	"fmt"
	"strings"

	"github.com/dominikbraun/graph"
	"github.com/peerdns/peerdns/pkg/storage"
)

// Store implements the graph.Store interface using MDBX for persistence.
// K is the type of the vertex identifier (e.g., string).
type Store[K comparable, T any] struct {
	storageProvider storage.Provider
	vertexPrefix    string
	edgePrefix      string
	ctx             context.Context // Internal context
}

// New creates a new MDBX-based graph store.
func New[K comparable, T any](sp storage.Provider, vertexPrefix, edgePrefix string, ctx context.Context) *Store[K, T] {
	return &Store[K, T]{
		storageProvider: sp,
		vertexPrefix:    vertexPrefix,
		edgePrefix:      edgePrefix,
		ctx:             ctx,
	}
}

// combinedVertex is used to serialize both the vertex value and its properties.
type combinedVertex[T any] struct {
	Value      T                      `json:"value"`
	Properties graph.VertexProperties `json:"properties"`
}

// combinedEdge is used to serialize edge properties.
type combinedEdge struct {
	Properties graph.EdgeProperties `json:"properties"`
}

// AddVertex adds a vertex to the store.
func (s *Store[K, T]) AddVertex(hash K, value T, properties graph.VertexProperties) error {
	key := fmt.Sprintf("%s:%v", s.vertexPrefix, hash)

	combined := combinedVertex[T]{
		Value:      value,
		Properties: properties,
	}

	combinedBytes, err := json.Marshal(combined)
	if err != nil {
		return fmt.Errorf("failed to marshal vertex data: %w", err)
	}

	if err := s.storageProvider.Set([]byte(key), combinedBytes); err != nil {
		return fmt.Errorf("failed to store vertex in MDBX: %w", err)
	}

	return nil
}

// Vertex retrieves a vertex from the store.
func (s *Store[K, T]) Vertex(hash K) (T, graph.VertexProperties, error) {
	key := fmt.Sprintf("%s:%v", s.vertexPrefix, hash)

	data, err := s.storageProvider.Get([]byte(key))
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return *new(T), graph.VertexProperties{}, graph.ErrVertexNotFound
		}
		return *new(T), graph.VertexProperties{}, errors.Wrap(err, "failed to retrieve vertex from MDBX")
	}

	var combined combinedVertex[T]
	if err := json.Unmarshal(data, &combined); err != nil {
		return *new(T), graph.VertexProperties{}, fmt.Errorf("failed to unmarshal vertex data: %w", err)
	}

	return combined.Value, combined.Properties, nil
}

// RemoveVertex removes a vertex and all its associated edges from the store.
func (s *Store[K, T]) RemoveVertex(hash K) error {
	vertexKey := fmt.Sprintf("%s:%v", s.vertexPrefix, hash)

	if err := s.storageProvider.Delete([]byte(vertexKey)); err != nil {
		return fmt.Errorf("failed to delete vertex from MDBX: %w", err)
	}

	outgoingEdgePrefix := fmt.Sprintf("%s:%v:", s.edgePrefix, hash)
	outgoingKeys, err := s.storageProvider.ListKeysWithPrefix(s.ctx, outgoingEdgePrefix)
	if err != nil {
		return fmt.Errorf("failed to list outgoing edges for vertex %v: %w", hash, err)
	}

	for _, edgeKey := range outgoingKeys {
		if err := s.storageProvider.Delete([]byte(edgeKey)); err != nil {
			return fmt.Errorf("failed to delete outgoing edge %s: %w", edgeKey, err)
		}
	}

	incomingEdgePrefix := fmt.Sprintf("%s:", s.edgePrefix)
	allEdgeKeys, err := s.storageProvider.ListKeysWithPrefix(s.ctx, incomingEdgePrefix)
	if err != nil {
		return fmt.Errorf("failed to list all edges for incoming edge removal: %w", err)
	}

	for _, edgeKey := range allEdgeKeys {
		parts := strings.Split(edgeKey, ":")
		if len(parts) != 3 {
			continue
		}
		if parts[2] == fmt.Sprintf("%v", hash) {
			if err := s.storageProvider.Delete([]byte(edgeKey)); err != nil {
				return fmt.Errorf("failed to delete incoming edge %s: %w", edgeKey, err)
			}
		}
	}

	return nil
}

// ListVertices lists all vertices in the store.
func (s *Store[K, T]) ListVertices() ([]K, error) {
	prefix := fmt.Sprintf("%s:", s.vertexPrefix)
	keys, err := s.storageProvider.ListKeysWithPrefix(s.ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list vertex keys: %w", err)
	}

	var vertices []K
	for _, key := range keys {
		idStr := strings.TrimPrefix(key, prefix)

		// Convert the string to type K
		id, err := s.decodeID(idStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode vertex ID: %w", err)
		}

		vertices = append(vertices, id)
	}

	return vertices, nil
}

// VertexCount returns the number of vertices in the store.
func (s *Store[K, T]) VertexCount() (int, error) {
	vertices, err := s.ListVertices()
	if err != nil {
		return 0, fmt.Errorf("failed to count vertices: %w", err)
	}
	return len(vertices), nil
}

// AddEdge adds an edge to the store.
func (s *Store[K, T]) AddEdge(sourceHash, targetHash K, edge graph.Edge[K]) error {
	key := fmt.Sprintf("%s:%v:%v", s.edgePrefix, sourceHash, targetHash)

	combined := combinedEdge{
		Properties: edge.Properties,
	}

	combinedBytes, err := json.Marshal(combined)
	if err != nil {
		return fmt.Errorf("failed to marshal edge data: %w", err)
	}

	if err := s.storageProvider.Set([]byte(key), combinedBytes); err != nil {
		return fmt.Errorf("failed to store edge in MDBX: %w", err)
	}

	return nil
}

// UpdateEdge updates an edge in the store.
func (s *Store[K, T]) UpdateEdge(sourceHash, targetHash K, edge graph.Edge[K]) error {
	key := fmt.Sprintf("%s:%v:%v", s.edgePrefix, sourceHash, targetHash)

	combined := combinedEdge{
		Properties: edge.Properties,
	}

	combinedBytes, err := json.Marshal(combined)
	if err != nil {
		return fmt.Errorf("failed to marshal updated edge data: %w", err)
	}

	if err := s.storageProvider.Set([]byte(key), combinedBytes); err != nil {
		return fmt.Errorf("failed to update edge in MDBX: %w", err)
	}

	return nil
}

// RemoveEdge removes an edge from the store.
func (s *Store[K, T]) RemoveEdge(sourceHash, targetHash K) error {
	key := fmt.Sprintf("%s:%v:%v", s.edgePrefix, sourceHash, targetHash)

	if err := s.storageProvider.Delete([]byte(key)); err != nil {
		return fmt.Errorf("failed to delete edge from MDBX: %w", err)
	}

	return nil
}

// Edge retrieves an edge from the store.
func (s *Store[K, T]) Edge(sourceHash, targetHash K) (graph.Edge[K], error) {
	key := fmt.Sprintf("%s:%v:%v", s.edgePrefix, sourceHash, targetHash)

	data, err := s.storageProvider.Get([]byte(key))
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return graph.Edge[K]{}, graph.ErrEdgeNotFound
		}
		return graph.Edge[K]{}, fmt.Errorf("failed to retrieve edge from MDBX: %w", err)
	}

	var combined combinedEdge
	if err := json.Unmarshal(data, &combined); err != nil {
		return graph.Edge[K]{}, fmt.Errorf("failed to unmarshal edge properties: %w", err)
	}

	return graph.Edge[K]{
		Source:     sourceHash,
		Target:     targetHash,
		Properties: combined.Properties,
	}, nil
}

// ListEdges lists all edges in the store.
func (s *Store[K, T]) ListEdges() ([]graph.Edge[K], error) {
	prefix := fmt.Sprintf("%s:", s.edgePrefix)
	keys, err := s.storageProvider.ListKeysWithPrefix(s.ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list edge keys: %w", err)
	}

	var edges []graph.Edge[K]
	for _, key := range keys {
		parts := strings.Split(strings.TrimPrefix(key, prefix), ":")
		if len(parts) != 2 {
			continue
		}

		// Decode the string key parts to type K
		source, err := s.decodeID(parts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to decode source ID: %w", err)
		}
		target, err := s.decodeID(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode target ID: %w", err)
		}

		edge, err := s.Edge(source, target)
		if err != nil {
			continue
		}
		edges = append(edges, edge)
	}

	return edges, nil
}

// EdgeCount returns the number of edges in the store.
func (s *Store[K, T]) EdgeCount() (int, error) {
	edges, err := s.ListEdges()
	if err != nil {
		return 0, fmt.Errorf("failed to count edges: %w", err)
	}
	return len(edges), nil
}
