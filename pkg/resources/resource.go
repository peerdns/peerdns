package resources

import (
	"github.com/peerdns/peerdns/pkg/observability"
	"sync"
)

// ResourceType represents the type of a resource.
type ResourceType string

// Predefined resource types for common resources used across services.
const (
	Database      ResourceType = "database"
	Storage       ResourceType = "storage"
	Identity      ResourceType = "identity"
	Observability ResourceType = "observability"
	Cache         ResourceType = "cache"
	Logger        ResourceType = "logger"
	Config        ResourceType = "config"
	Node          ResourceType = "node"
)

// Manager manages the registration and retrieval of resources.
type Manager struct {
	mu        sync.RWMutex
	obs       *observability.Observability
	resources map[ResourceType]interface{}
}

// NewManager creates a new ResourceManager.
func NewManager(obs *observability.Observability) *Manager {
	return &Manager{
		obs:       obs,
		resources: make(map[ResourceType]interface{}),
	}
}

// Register registers a resource with the specified type.
func (rm *Manager) Register(resourceType ResourceType, resource interface{}) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.resources[resourceType] = resource
}

// Get retrieves a registered resource by its type.
func (rm *Manager) Get(resourceType ResourceType) (interface{}, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	resource, exists := rm.resources[resourceType]
	return resource, exists
}

// R retrieves a resource of a specific type (e.g., *Database) from the Manager.
func R[T any](resourceType ResourceType) (T, bool) {
	if res, ok := global.Get(resourceType); ok {
		if typedRes, ok := res.(T); ok {
			return typedRes, true
		}
	}
	var zeroValue T
	return zeroValue, false
}
