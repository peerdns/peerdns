package gateway

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/runtime"
	"golang.org/x/sync/errgroup"
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
	s.logger.Info("Starting gateway service")
	g, ctx := errgroup.WithContext(s.ctx)
	_ = ctx
	return g.Wait()
}

func (s *Service) Stop() error {
	s.logger.Info("Stopping gateway service")
	return nil
}
