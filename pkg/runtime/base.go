package runtime

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/identity"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/peerdns/peerdns/pkg/resources"
	"github.com/peerdns/peerdns/pkg/shutdown"
	"github.com/peerdns/peerdns/pkg/storage"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// BaseService provides the foundational service management and coordination functionality for the runtime system.
// It manages the lifecycle of multiple services, coordinates startup and shutdown, and handles shared dependencies
// through the Resource Manager (rm) and state tracking through the ServiceStateManager (SSM).
//
// The BaseService struct is central to starting, stopping, and monitoring all services in the runtime. It provides
// an entry point for the runtime’s service management functions and integrates with various runtime components.
type BaseService struct {
	ctx      context.Context    // Application root context used for context propagation.
	config   *config.Config     // Application configuration containing all necessary settings.
	logger   logger.Logger      // Logger instance used for logging throughout the BaseService and its services.
	shutdown *shutdown.Manager  // Shutdown manager for handling graceful shutdown and cleanup.
	rm       *resources.Manager // Resource manager to manage shared dependencies across services.
	sm       *storage.Manager
	im       *identity.Manager
	obs      *observability.Observability
}

// NewBaseService creates and initializes a new BaseService instance.
// The BaseService provides a unified way to manage service lifecycles, shared dependencies, and logging.
//
// Parameters:
//   - ctx (context.Context): The root context for the application.
//   - config (*config.Config): Application configuration object containing necessary settings.
//   - logger (logger.Logger): Logger instance used for logging across services.
//   - rm (*resources.Manager): Resource manager for managing shared resources and dependencies across services.
//   - shutdown (*shutdown.Manager): Shutdown manager to handle graceful service shutdown.
//
// Returns:
//   - (*BaseService, error): Returns the initialized BaseService instance and any error encountered during initialization.
func NewBaseService(ctx context.Context, config *config.Config, logger logger.Logger, rm *resources.Manager, obs *observability.Observability, shutdown *shutdown.Manager) (*BaseService, error) {
	// Initialize storage manager. Storage itself should be globally initialized as there's no
	// service that will not use it on this way or another.
	// TODO: Read only mechanisms are missing.
	storageManager, err := storage.NewManager(ctx, config.Mdbx)
	if err != nil {
		return nil, err
	}

	// Register storage into the resource management for future use.
	rm.Register(resources.Storage, storageManager)

	// Initialize global identity manager.
	// Identities will be necessary throughout different applications.
	identityManager, imErr := identity.NewManager(&config.Identity, logger)
	if imErr != nil {
		return nil, imErr
	}
	rm.Register(resources.Identity, identityManager)

	// Register observability into the resource manager for potential use by the system...
	rm.Register(resources.Observability, obs)

	return &BaseService{
		ctx:      ctx,
		config:   config,
		logger:   logger,
		rm:       rm,
		shutdown: shutdown,
		sm:       storageManager,
		im:       identityManager,
		obs:      obs,
	}, nil
}

func (s *BaseService) ShutdownManager() *shutdown.Manager {
	return s.shutdown
}

func (s *BaseService) Observability() *observability.Observability {
	return s.obs
}

func (s *BaseService) StorageManager() *storage.Manager {
	return s.sm
}

func (s *BaseService) IdentityManager() *identity.Manager {
	return s.im
}

// Start initiates the startup sequence for the specified services concurrently.
// It ensures that all requested services exist in the registration storage, and if any are missing,
// it returns an error. Each service's state is tracked and updated using the global ServiceStateManager (SSM).
//
// This function uses `errgroup` to manage concurrent startup of services and ensures that if any service
// fails to start, the overall startup sequence is halted, and an error is returned.
//
// Parameters:
//   - services (...ServiceType): A variadic list of service types to be started.
//
// Returns:
//   - error: Returns nil if all services start successfully, or an error if any service fails to start.
func (s *BaseService) Start(services ...ServiceType) error {
	s.logger.Info("Starting up the runtime services", zap.Any("services", services))

	// Check if the requested services exist in the registration storage.
	if missingService, exists := Exists(services...); !exists {
		return errors.Errorf("requested service %s cannot be started as it does not exist", missingService)
	}

	// Initialize an errgroup and context for starting the services concurrently.
	g, _ := errgroup.WithContext(s.ctx)

	// Track all services that need to be started and register dependencies.
	for _, service := range services {
		g.Go(func() error {
			// Retrieve the registered service and start it.
			if rService, ok := Get(service); ok {
				// Notify listeners that this service is in the Initializing state.
				ssm.SetState(service, Initializing)

				// Register a shutdown callback to ensure services are gracefully stopped.
				s.shutdown.AddShutdownCallback(func() error {
					if err := rService.Stop(); err != nil {
						// If stopping the service fails, set its state to Failed.
						ssm.SetState(service, Failed)
						return err
					}
					// Set service state to Stopped on successful shutdown.
					ssm.SetState(service, Stopped)
					return nil
				})

				// Start the service and handle any errors.
				err := rService.Start()
				if err != nil {
					// Update state to Failed if the service startup fails.
					ssm.SetState(service, Failed)
					return errors.Wrapf(err, "failed to start service %s", service)
				}

				// Service has started successfully, update the state to Started.
				ssm.SetState(service, Started)
				s.logger.Info("Service successfully started", zap.String("service", service.String()))
			} else {
				return errors.Errorf("service %s not found in registry", service)
			}
			return nil
		})
	}

	// Wait for all services to complete their startup.
	return g.Wait()
}
