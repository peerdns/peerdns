package sequencer

import (
	"fmt"
	"sync"
	"time"

	"github.com/peerdns/peerdns/pkg/logger"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// StateType represents the type of the Sequencer service.
type StateType string

func (s StateType) String() string {
	return string(s)
}

// Define Sequencer state types.
const (
	SequencerStateType StateType = "sequencer"
	ProducerStateType  StateType = "producer"
)

// State represents the current state of the Sequencer.
type State int

// Define various states a Sequencer can be in.
const (
	Uninitialized State = iota // The Sequencer has not been initialized yet.
	Initializing               // The Sequencer is currently initializing.
	Initialized                // The Sequencer has been successfully initialized.
	Starting                   // The Sequencer is starting its operations.
	Started                    // The Sequencer has started successfully and is running.
	Produced
	Failed   // The Sequencer failed to initialize or start.
	Stopping // The Sequencer is in the process of stopping.
	Stopped  // The Sequencer has been stopped.
)

// String returns a string representation of the Sequencer state.
func (s State) String() string {
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
	case Produced:
		return "produced"
	case Failed:
		return "failed"
	case Stopping:
		return "stopping"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// StateManager manages and tracks the state of the Sequencer.
type StateManager struct {
	mu            sync.RWMutex
	logger        logger.Logger
	metrics       *SequencerMetrics
	serviceStates map[StateType]State
	waitChans     map[StateType]chan struct{}
}

// SequencerMetrics holds all the metrics instruments for the Sequencer.
type SequencerMetrics struct {
	StateGauge metric.Int64ObservableGauge
}

// InitializeSequencerMetrics initializes the metrics instruments for the Sequencer.
func InitializeSequencerMetrics(meter metric.Meter) (*SequencerMetrics, error) {
	m := &SequencerMetrics{}
	var err error

	// Create an Int64ObservableGauge for tracking Sequencer states.
	m.StateGauge, err = meter.Int64ObservableGauge(
		"sequencer.state",
		metric.WithDescription("Tracks the current state of the Sequencer"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// NewStateManager creates a new StateManager instance with the provided logger and metrics.
func NewStateManager(logger logger.Logger, metrics *SequencerMetrics) *StateManager {
	manager := &StateManager{
		logger:        logger,
		metrics:       metrics,
		serviceStates: make(map[StateType]State),
		waitChans:     make(map[StateType]chan struct{}),
	}

	// Initialize metrics callback if metrics are provided.
	if metrics != nil {
		// Assuming you have an OpenTelemetry Meter setup elsewhere.
		// This is a placeholder for registering the callback.
		// You would need to integrate this with your observability setup.
		// Example:
		// _, err := meter.RegisterCallback(
		//     manager.recordStateCallback,
		//     metrics.StateGauge,
		// )
		// Handle err if needed.
	}

	return manager
}

// SetState updates the state of the Sequencer, logs the change, and emits a metric.
func (sm *StateManager) SetState(st StateType, state State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	previousState, exists := sm.serviceStates[st]
	if !exists {
		previousState = Uninitialized
	}
	sm.serviceStates[st] = state

	// Log the state change.
	sm.logger.Info("Sequencer state changed",
		zap.String("previous_state", previousState.String()),
		zap.String("new_state", state.String()),
	)

	// Emit a state change metric if metrics are available.
	if sm.metrics != nil {
		// Record the current state with attributes.
		// You would need to integrate this with your observability setup.
		// Example:
		// sm.metrics.StateGauge.Observe(context.Background(), int64(state), attribute.String("sequencer", "state"))
	}

	// Notify any goroutines waiting for this state.
	if state == Started || state == Stopped || state == Failed {
		if ch, exists := sm.waitChans[st]; exists {
			close(ch)
			delete(sm.waitChans, st)
		}
	}
}

// GetState retrieves the current state of the Sequencer.
func (sm *StateManager) GetState(st StateType) State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.serviceStates[st]
}

// WaitForState waits for the Sequencer to reach the desired state within a timeout.
func (sm *StateManager) WaitForState(st StateType, desiredState State, timeout time.Duration) error {
	sm.mu.Lock()
	currentState, exists := sm.serviceStates[st]
	if exists && currentState == desiredState {
		sm.mu.Unlock()
		return nil
	}

	// If the channel doesn't exist, create one.
	ch, exists := sm.waitChans[st]
	if !exists {
		ch = make(chan struct{})
		sm.waitChans[st] = ch
	}
	sm.mu.Unlock()

	select {
	case <-ch:
		sm.mu.RLock()
		defer sm.mu.RUnlock()
		if sm.serviceStates[st] == desiredState {
			return nil
		}
		return fmt.Errorf("sequencer did not reach desired state: %s", desiredState.String())
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for sequencer to reach state: %s", desiredState.String())
	}
}
