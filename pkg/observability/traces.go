package observability

import (
	"context"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// InitTracer initializes OpenTelemetry tracing.
func InitTracer(ctx context.Context, cfg config.TracingConfig, logger logger.Logger) (trace.Tracer, *sdktrace.TracerProvider, error) {
	if !cfg.Enable {
		return otel.Tracer("disabled"), nil, nil
	}

	// Create OTLP trace exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithHeaders(cfg.Headers),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		logger.Error("Failed to create OTLP trace exporter", zap.Error(err))
		return nil, nil, err
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("peerdns"),
		),
	)
	if err != nil {
		logger.Error("Failed to create resource for tracing", zap.Error(err))
		return nil, nil, err
	}

	// Determine sampler
	var sampler sdktrace.Sampler
	switch cfg.Sampler {
	case "always_on":
		sampler = sdktrace.AlwaysSample()
	case "probability":
		sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRate)
	default:
		sampler = sdktrace.AlwaysSample()
		logger.Warn("Unknown sampler type, defaulting to AlwaysSample")
	}

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("peerdns")

	return tracer, tp, nil
}
