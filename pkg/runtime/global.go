package runtime

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/peerdns/peerdns/pkg/resources"
	"github.com/peerdns/peerdns/pkg/shutdown"
)

// Global variables that provide access to core runtime components.
var (
	ssm    *ServiceStateManager // ssm holds a global instance of the ServiceStateManager for service state tracking.
	global *BaseService         // global is a reference to the globally initialized BaseService instance.
)

// InitializeBaseService initializes and returns a new BaseService instance.
// It also initializes the global service state manager (SSM) for cross-service state signaling and stores the
// reference to the global BaseService. This function should be called only once during the application startup.
//
// Parameters:
//   - ctx (context.Context): The root context for the application.
//   - config (*config.Config): The application's configuration object.
//   - logger (logger.Logger): The logger instance to be used for logging across services.
//   - rm (*resources.Manager): The resource manager for managing shared resources across services.
//   - shutdown (*shutdown.Manager): The shutdown manager to handle graceful shutdown of services.
//
// Returns:
//   - (*BaseService, error): Returns the initialized BaseService instance and any error encountered during initialization.
func InitializeBaseService(ctx context.Context, config *config.Config, logger logger.Logger, rm *resources.Manager, obs *observability.Observability, shutdown *shutdown.Manager) (*BaseService, error) {
	// Initialize state manager for cross-service state signaling
	ssm = NewServiceStateManager(logger, obs)

	// Construct base service that every service has to implement to satisfy the requirements of this system.
	base, bErr := NewBaseService(ctx, config, logger, rm, obs, shutdown)
	if bErr != nil {
		return nil, bErr
	}

	// Set the global BaseService reference.
	global = base

	return base, nil
}

// G returns the globally initialized BaseService instance.
// This function provides a convenient way to access the BaseService from anywhere within the application.
//
// Returns:
//   - (*BaseService): Returns the global BaseService instance or nil if it hasn't been initialized.
func G() *BaseService {
	return global
}

// SSM returns the global ServiceStateManager instance.
// This function provides access to the global ServiceStateManager, which tracks the state of all services
// and allows for cross-service state signaling.
//
// Returns:
//   - (*ServiceStateManager): Returns the global ServiceStateManager instance or nil if it hasn't been initialized.
func SSM() *ServiceStateManager {
	return ssm
}
