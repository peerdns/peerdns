package chain

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/node"
	"github.com/peerdns/peerdns/pkg/resources"
	"github.com/peerdns/peerdns/pkg/runtime"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	base   *runtime.BaseService
	ctx    context.Context
	cfg    *config.Config
	logger logger.Logger
	dNode  *node.Node
}

func NewService(ctx context.Context, logger logger.Logger, cfg *config.Config, base *runtime.BaseService) (*Service, error) {
	// Node is basically a wrapper around consensus, chain, identity management, peer system and peer discovery system.
	// Not to forget metrics and ping-pong game between peers to establish metrics baseline.
	dNode, dnErr := node.NewNode(ctx, cfg, logger, base.StorageManager(), base.IdentityManager())
	if dnErr != nil {
		return nil, errors.Wrap(dnErr, "failed to initialize node")
	}

	// Be sure that on shutdown process node is gracefully stopped.
	base.ShutdownManager().AddShutdownCallback(func() error {
		return dNode.Shutdown()
	})

	// Register node to the global resource manager for future consumption by other services.
	resources.G().Register(resources.Node, dNode)

	return &Service{
		ctx:    ctx,
		cfg:    cfg,
		logger: logger,
		base:   base,
		dNode:  dNode,
	}, nil
}

func (s *Service) Type() runtime.ServiceType {
	return ServiceType
}

func (s *Service) Start() error {
	s.logger.Info("Starting chain service")
	g, ctx := errgroup.WithContext(s.ctx)
	_ = ctx
	return g.Wait()
}

func (s *Service) Stop() error {
	s.logger.Info("Stopping chain service")
	return nil
}
