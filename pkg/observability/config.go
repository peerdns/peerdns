package observability

import (
	"github.com/peerdns/peerdns/pkg/config"
)

// ObservabilityConfig holds the configuration for the observability package.
type ObservabilityConfig struct {
	Metrics config.MetricsConfig `yaml:"metrics"`
	Tracing config.TracingConfig `yaml:"tracing"`
}

// LoadObservabilityConfig extracts the observability configuration from the global config.
func LoadObservabilityConfig(cfg *config.Config) ObservabilityConfig {
	return ObservabilityConfig{
		Metrics: cfg.Observability.Metrics,
		Tracing: cfg.Observability.Tracing,
	}
}
