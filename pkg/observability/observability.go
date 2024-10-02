package observability

import (
	"context"
	"github.com/davecgh/go-spew/spew"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"

	"github.com/peerdns/peerdns/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Observability encapsulates metrics, tracing, and logging.
type Observability struct {
	Logger         logger.Logger
	Meter          metric.Meter
	Tracer         trace.Tracer
	TracerProvider *sdktrace.TracerProvider
	ServiceName    string
}

// Init initializes all observability components based on the provided configuration.
func Init(ctx context.Context, cfg ObservabilityConfig, logger logger.Logger) (*Observability, error) {
	var obs Observability
	var err error

	obs.Logger = logger
	obs.ServiceName = "peerdns" // You can make this configurable if needed

	spew.Dump(cfg)

	// Initialize Metrics
	if cfg.Metrics.Enable {

		obs.Meter, err = InitMetrics(ctx, cfg.Metrics, logger)
		if err != nil {
			obs.Logger.Error("Failed to initialize metrics", zap.Error(err))
			return nil, err
		}

		// Initialize metrics instruments
		err = InitializeMetricsInObservability(ctx, obs.Meter, logger)
		if err != nil {
			obs.Logger.Error("Failed to initialize metrics instruments", zap.Error(err))
			return nil, err
		}
	} else {
		obs.Meter = otel.Meter("disabled")
	}

	// Initialize Tracing
	if cfg.Tracing.Enable {
		obs.Tracer, obs.TracerProvider, err = InitTracer(ctx, cfg.Tracing, logger)
		if err != nil {
			obs.Logger.Error("Failed to initialize tracer", zap.Error(err))
			return nil, err
		}
	} else {
		obs.Tracer = otel.Tracer("disabled")
	}

	return &obs, nil
}

// Shutdown gracefully shuts down the observability providers.
func (o *Observability) Shutdown(ctx context.Context) error {
	var err error
	if o.TracerProvider != nil {
		if shutdownErr := o.TracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = shutdownErr
		}
	}
	return err
}

// RecordServiceStateChange records a service state change event.
func (o *Observability) RecordServiceStateChange(ctx context.Context, serviceType, state string) {
	if ServiceStateCounter != nil {
		ServiceStateCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("service_type", serviceType),
			attribute.String("state", state),
		))
	}
}

// EmitServiceStateChangeTrace emits a trace when a service state changes.
func (o *Observability) EmitServiceStateChangeTrace(ctx context.Context, serviceType, previousState, newState string) {
	_, span := o.Tracer.Start(ctx, "ServiceStateChange")
	span.SetAttributes(
		attribute.String("service_type", serviceType),
		attribute.String("previous_state", previousState),
		attribute.String("new_state", newState),
	)
	span.End()
}
