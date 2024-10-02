package observability

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
)

var (
	global *Observability
)

func Initialize(ctx context.Context, cfg *config.Config, logger logger.Logger) (*Observability, error) {
	obs, obsErr := New(ctx, cfg, logger)
	if obsErr != nil {
		return nil, obsErr
	}
	global = obs
	return obs, nil
}

func G() *Observability {
	return global
}
