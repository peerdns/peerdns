package consensus

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/node"
	"github.com/peerdns/peerdns/pkg/resources"
	"github.com/peerdns/peerdns/pkg/runtime"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"time"
)

type Service struct {
	ctx    context.Context
	cfg    *config.Config
	logger logger.Logger
	base   *runtime.BaseService
}

func NewService(ctx context.Context, logger logger.Logger, cfg *config.Config, base *runtime.BaseService) (*Service, error) {
	return &Service{
		ctx:    ctx,
		cfg:    cfg,
		logger: logger,
		base:   base,
	}, nil
}

func (s *Service) Type() runtime.ServiceType {
	return ServiceType
}

func (s *Service) Start() error {
	s.logger.Info("Starting consensus service")

	// Chain service needs to be started so that consensus can operate efficiently.
	// Prior to starting consensus service, first, we are going to ensure that chain is here or fail after 10s if not..
	err := runtime.SSM().WaitForState(runtime.ChainServiceType, runtime.Started, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "Consensus service cannot be started because the chain service is not started")
	}

	dNode, dnFound := resources.R[*node.Node](resources.Node)
	if !dnFound {
		return errors.New("failure to discover global node resource")
	}

	g, _ := errgroup.WithContext(s.ctx)

	g.Go(func() error {
		return dNode.Validator().Start()
	})

	return g.Wait()
}

func (s *Service) Stop() error {
	s.logger.Info("Stopping consensus service")
	return nil
}
