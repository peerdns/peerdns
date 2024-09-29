// pkg/shutdown/shutdown.go
package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/peerdns/peerdns/pkg/logger"
	"go.uber.org/zap"
)

// ShutdownManager handles graceful shutdown of the application.
type ShutdownManager struct {
	ctx       context.Context
	cancel    context.CancelFunc
	logger    logger.Logger
	wg        sync.WaitGroup
	signals   []os.Signal
	once      sync.Once
	callbacks []func()
	mu        sync.Mutex
}

// NewShutdownManager creates a new ShutdownManager.
// It accepts a parent context, a logger, and optional OS signals to listen for.
func NewShutdownManager(ctx context.Context, logger logger.Logger, signals ...os.Signal) *ShutdownManager {
	ctx, cancel := context.WithCancel(ctx)
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}
	return &ShutdownManager{
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		signals: signals,
	}
}

// Context returns the context associated with the ShutdownManager.
func (sm *ShutdownManager) Context() context.Context {
	return sm.ctx
}

// AddShutdownCallback registers a callback function to be called during shutdown.
func (sm *ShutdownManager) AddShutdownCallback(callback func()) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.callbacks = append(sm.callbacks, callback)
}

// Start begins listening for OS signals to initiate shutdown.
func (sm *ShutdownManager) Start() {
	sm.wg.Add(1)
	go sm.handleSignals()
}

// handleSignals listens for OS signals and initiates shutdown when received.
func (sm *ShutdownManager) handleSignals() {
	defer sm.wg.Done()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sm.signals...)

	select {
	case sig := <-sigChan:
		sm.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		sm.shutdown()
	case <-sm.ctx.Done():
		sm.logger.Info("Context canceled, shutting down")
		sm.shutdown()
	}
}

// shutdown performs the actual shutdown sequence, ensuring it's only executed once.
func (sm *ShutdownManager) shutdown() {
	sm.once.Do(func() {
		sm.cancel()
		sm.mu.Lock()
		defer sm.mu.Unlock()
		for _, callback := range sm.callbacks {
			callback()
		}
	})
}

// Wait blocks until the shutdown sequence is complete.
func (sm *ShutdownManager) Wait() {
	sm.wg.Wait()
}
