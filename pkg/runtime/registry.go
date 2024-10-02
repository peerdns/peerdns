package runtime

import (
	"context"
	"github.com/pkg/errors"
	"sync"
)

// ServiceType represents the type of service.
type ServiceType string

// String returns the string representation of the ServiceType.
func (s ServiceType) String() string {
	return string(s)
}

// RegisterHandlerFn is a function that registers a service.
type RegisterHandlerFn func(ctx context.Context, base *BaseService) (Service, error)

var (
	ChainServiceType     ServiceType = "chain"
	ConsensusServiceType ServiceType = "consensus"
	DNSServiceType       ServiceType = "dns"
	RouterServiceType    ServiceType = "router"
	HealthServiceType    ServiceType = "health"
	GatewayServiceType   ServiceType = "gateway"

	// mutex protects access to the registry.
	mutex sync.RWMutex
	// registry is a map that stores registered services, including default supported services.
	registry = map[ServiceType]Service{}

	// handlerMutex protects access to the handler registry.
	handlerMutex sync.RWMutex
	// handlers is a map that stores service type to register handler functions.
	handlers = map[ServiceType]RegisterHandlerFn{}
)

// Get retrieves a service from the registry.
// It returns the service and a boolean indicating whether the service was found.
func Get(service ServiceType) (Service, bool) {
	mutex.RLock()
	defer mutex.RUnlock()
	srvce, exists := registry[service]
	return srvce, exists
}

// Register adds a service to the registry if it doesn't already exist.
// It returns true if the service was successfully registered, false otherwise.
func Register(service ServiceType, srvce Service) bool {
	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := registry[service]; ok {
		return false
	}
	registry[service] = srvce
	return true
}

// Exists checks if a service exists in the registry.
// It returns true if the service exists, false otherwise.
func Exists(service ...ServiceType) (*ServiceType, bool) {
	mutex.RLock()
	defer mutex.RUnlock()
	for _, v := range service {
		if _, exists := registry[v]; !exists {
			return &v, false
		}
	}

	return nil, true
}

// ListAvailableServices retrieves a list of all registered services.
func ListAvailableServices() []ServiceType {
	mutex.RLock()
	defer mutex.RUnlock()
	services := make([]ServiceType, 0, len(registry))
	for service, _ := range registry {
		services = append(services, service)
	}
	return services
}

// Remove removes a service from the registry.
func Remove(service ServiceType) {
	mutex.Lock()
	defer mutex.Unlock()
	delete(registry, service)
}

// RegisterHandler registers a handler for a specific service type.
func RegisterHandler(service ServiceType, handler RegisterHandlerFn) bool {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()

	if _, ok := handlers[service]; ok {
		return false
	}
	handlers[service] = handler
	return true
}

// InjectServices executes all registered handlers to initialize services.
// It returns an error if any handler fails.
func InjectServices(ctx context.Context, base *BaseService) error {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()

	for service, handler := range handlers {
		if _, err := handler(ctx, base); err != nil {
			return errors.Wrap(err, "failed to register service "+service.String())
		}
	}

	return nil
}
