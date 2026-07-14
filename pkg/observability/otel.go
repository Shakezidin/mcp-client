package observability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// Config defines OpenTelemetry settings for the MCP client.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Endpoint       string
	Insecure       bool
	OTLPPort       int
}

// DefaultConfig returns a reasonable local observability configuration.
func DefaultConfig() Config {
	return Config{
		ServiceName:    "aib-mcp-client",
		ServiceVersion: "v1.0.0",
		Environment:    "local",
		Endpoint:       "localhost",
		Insecure:       true,
		OTLPPort:       4317,
	}
}

// Setup initializes OTel tracing, metrics, and logging.
// It returns a shutdown function that should be called once on application exit.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "localhost"
	}
	endpoint = fmt.Sprintf("%s:%d", endpoint, cfg.OTLPPort)

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(traceProvider)

	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(3*time.Second),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	logExporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(endpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(logProvider)

	shutdown := func(ctx context.Context) error {
		var shutdownErr error
		shutdownErr = errors.Join(shutdownErr, traceProvider.Shutdown(ctx))
		shutdownErr = errors.Join(shutdownErr, meterProvider.Shutdown(ctx))
		shutdownErr = errors.Join(shutdownErr, logProvider.Shutdown(ctx))
		return shutdownErr
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return shutdown, nil
}
