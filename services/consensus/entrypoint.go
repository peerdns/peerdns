package consensus

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/runtime"
	"github.com/pkg/errors"
	"sync"
)

var (
	// ServiceType defines the type of this service.
	ServiceType = runtime.ConsensusServiceType

	// once ensures that the handler registration happens only once.
	once sync.Once
)

func ServiceHandler(ctx context.Context, baseService *runtime.BaseService) (runtime.Service, error) {
	gLog := logger.G()
	gLog.Debug("Registering consensus service...")

	service, sErr := NewService(ctx, gLog, config.G(), baseService)
	if sErr != nil {
		return nil, errors.Wrap(sErr, "Failed to create new consensus service")
	}

	if registered := runtime.Register(ServiceType, service); !registered {
		return nil, errors.Errorf("Failed to register consensus service")
	}

	return nil, nil
}

func init() {
	once.Do(func() {
		// At this point attempt to register consensus handler.
		// In case that there are any errors during this process, hard panic as a system is heavily corrupted.
		if loaded := runtime.RegisterHandler(ServiceType, ServiceHandler); !loaded {
			panic("Consensus service could not be registered")
		}
	})
}
