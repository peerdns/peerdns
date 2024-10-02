package runtime

import (
	"context"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"sync"
	"time"
)

// ServiceState represents the current state of a service.
type ServiceState int

const (
	Uninitialized ServiceState = iota // The service is not yet initialized.
	Initializing                      // The service is currently initializing.
	Initialized                       // The service has been initialized.
	Starting                          // The service is in the process of starting.
	Started                           // The service has started successfully.
	Failed                            // The service failed to initialize or start.
	Stopped                           // The service was stopped.
)

// String returns a string representation of the ServiceState.
func (s ServiceState) String() string {
	switch s {
	case Uninitialized:
		return "uninitialized"
	case Initializing:
		return "initializing"
	case Initialized:
		return "initialized"
	case Starting:
		return "starting"
	case Started:
		return "started"
	case Failed:
		return "failed"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ServiceStateManager manages the state of services.
type ServiceStateManager struct {
	mu                sync.RWMutex
	logger            logger.Logger
	obs               *observability.Observability
	serviceStates     map[ServiceType]ServiceState
	waitChans         map[ServiceType]chan struct{} // Channels to signal state transitions
	serviceStateGauge metric.Int64ObservableGauge   // Observable gauge metric for tracking service states
}

// NewServiceStateManager creates a new ServiceStateManager.
func NewServiceStateManager(logger logger.Logger, obs *observability.Observability) *ServiceStateManager {
	manager := &ServiceStateManager{
		logger:        logger,
		obs:           obs,
		serviceStates: make(map[ServiceType]ServiceState),
		waitChans:     make(map[ServiceType]chan struct{}),
	}

	// Initialize the service state gauge metric
	manager.initMetrics()

	return manager
}

// initMetrics initializes the OpenTelemetry metrics for the ServiceStateManager.
func (sm *ServiceStateManager) initMetrics() {
	if sm.obs != nil && sm.obs.Meter != nil {
		// Create an Int64ObservableGauge for service states
		gauge, err := sm.obs.Meter.Int64ObservableGauge(
			"service_state",
			metric.WithDescription("Tracks the current state of each service"),
		)
		if err != nil {
			sm.logger.Error("Failed to create Int64ObservableGauge", zap.Error(err))
			return
		}
		sm.serviceStateGauge = gauge

		// Register a callback to update the gauge value for each service
		_, err = sm.obs.Meter.RegisterCallback(
			sm.recordServiceStatesCallback, // Correctly reference the method here
			sm.serviceStateGauge,
		)
		if err != nil {
			sm.logger.Error("Failed to register callback for service states", zap.Error(err))
		}
	}
}

// recordServiceStatesCallback is the callback function that updates the gauge values.
func (sm *ServiceStateManager) recordServiceStatesCallback(ctx context.Context, observer metric.Observer) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for service, state := range sm.serviceStates {
		observer.ObserveInt64(sm.serviceStateGauge, int64(state), metric.WithAttributes(
			attribute.String("service", string(service)),
			attribute.String("state", state.String()),
		))
	}
	return nil
}

// SetState updates the state of a service and emits a metric.
func (sm *ServiceStateManager) SetState(service ServiceType, state ServiceState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get the current state to determine if it has changed
	previousState, exists := sm.serviceStates[service]
	if !exists {
		previousState = Uninitialized // Assume Uninitialized if the service is not found
	}

	// Update the service state
	sm.serviceStates[service] = state

	// Log state change
	sm.logger.Debug("Service state changed",
		zap.String("type", string(service)),
		zap.String("previous", previousState.String()),
		zap.String("new", state.String()),
	)

	// Emit state change event as a metric
	if sm.obs != nil {
		sm.obs.RecordServiceStateChange(context.Background(), service.String(), state.String())
	}

	// Notify any waiters if the service has reached the desired state
	if state == Started {
		if ch, exists := sm.waitChans[service]; exists {
			close(ch)
			delete(sm.waitChans, service)
		}
	}
}

// GetState retrieves the current state of a service.
func (sm *ServiceStateManager) GetState(service ServiceType) ServiceState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.serviceStates[service]
}

// WaitForState waits for a service to reach a specified state, with a timeout.
func (sm *ServiceStateManager) WaitForState(service ServiceType, desiredState ServiceState, timeout time.Duration) error {
	sm.mu.Lock()
	state, exists := sm.serviceStates[service]

	// If the service doesn't exist in the state map, create a new channel for it
	if !exists {
		sm.logger.Debug("Service not found in state map, waiting for it to be added",
			zap.String("type", string(service)),
			zap.String("state", desiredState.String()),
		)

		// Create a new channel to wait for the service to be added and update its state
		ch := make(chan struct{})
		sm.waitChans[service] = ch
		sm.mu.Unlock()

		// Wait for the service to be added to the state map or for the timeout to expire
		select {
		case <-ch:
			// Recheck the state after the service is added
			sm.mu.RLock()
			state = sm.serviceStates[service]
			sm.mu.RUnlock()
		case <-time.After(timeout):
			sm.logger.Error("Service did not appear in state map within timeout - forgotten to start the service?",
				zap.String("type", string(service)),
				zap.String("state", desiredState.String()),
				zap.Duration("timeout", timeout),
			)
			return errors.Errorf("service %s did not appear within %v", service, timeout)
		}
	} else {
		sm.mu.Unlock()
	}

	// If the service is already in the desired state, return immediately
	if state == desiredState {
		sm.logger.Debug("Service already in desired state",
			zap.String("type", string(service)),
			zap.String("state", desiredState.String()),
		)
		return nil
	}

	// Create a new channel to wait for the state change if it doesn't already exist
	sm.mu.Lock()
	ch, exists := sm.waitChans[service]
	if !exists {
		ch = make(chan struct{})
		sm.waitChans[service] = ch
	}
	sm.mu.Unlock()

	// Wait for the state to change or for the timeout to expire
	sm.logger.Debug("Waiting for service to reach state",
		zap.String("type", string(service)),
		zap.String("state", desiredState.String()),
		zap.Duration("timeout", timeout),
	)

	select {
	case <-ch:
		sm.logger.Debug("Service reached desired state",
			zap.String("type", string(service)),
			zap.String("state", desiredState.String()),
		)
		return nil // Desired state reached
	case <-time.After(timeout):
		sm.logger.Error("Service did not reach desired state within timeout",
			zap.String("type", string(service)),
			zap.String("state", desiredState.String()),
			zap.Duration("timeout", timeout),
		)
		return errors.Errorf("service %s did not reach state %s within %v", service, desiredState, timeout)
	}
}
