package observability

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
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
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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

	endpoint := resolveEndpoint(cfg.Endpoint, cfg.OTLPPort)
	traceDialOpts := buildOTLPDialOptions(cfg.Insecure)

	if err := waitForCollector(endpoint, 2*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "otel collector unavailable at %s; falling back to no-op telemetry: %v\n", endpoint, err)
		return setupNoopTelemetry(ctx, res)
	}

	traceExporterOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(traceDialOpts...),
	}
	if cfg.Insecure {
		traceExporterOpts = append(traceExporterOpts, otlptracegrpc.WithInsecure())
	}

	traceExporter, err := otlptracegrpc.New(ctx, traceExporterOpts...)
	if err != nil {
		return setupNoopTelemetry(ctx, res)
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(traceProvider)

	metricExporterOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithDialOption(traceDialOpts...),
	}
	if cfg.Insecure {
		metricExporterOpts = append(metricExporterOpts, otlpmetricgrpc.WithInsecure())
	}

	metricExporter, err := otlpmetricgrpc.New(ctx, metricExporterOpts...)
	if err != nil {
		return setupNoopTelemetry(ctx, res)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(3*time.Second),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	logExporterOpts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(endpoint),
		otlploggrpc.WithDialOption(traceDialOpts...),
	}
	if cfg.Insecure {
		logExporterOpts = append(logExporterOpts, otlploggrpc.WithInsecure())
	}

	logExporter, err := otlploggrpc.New(ctx, logExporterOpts...)
	if err != nil {
		return setupNoopTelemetry(ctx, res)
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

func resolveEndpoint(endpoint string, port int) string {
	if endpoint == "" {
		return fmt.Sprintf("localhost:%d", port)
	}

	if strings.Contains(endpoint, ":") {
		if _, _, err := net.SplitHostPort(endpoint); err == nil {
			return endpoint
		}
	}

	return fmt.Sprintf("%s:%d", endpoint, port)
}

func buildOTLPDialOptions(insecureTransport bool) []grpc.DialOption {
	if insecureTransport {
		return []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		}
	}

	return []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})),
		grpc.WithBlock(),
	}
}

func waitForCollector(endpoint string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		return err
	}
	return conn.Close()
}

func setupNoopTelemetry(ctx context.Context, res *resource.Resource) (func(context.Context) error, error) {
	otel.SetTracerProvider(trace.NewNoopTracerProvider())
	otel.SetMeterProvider(sdkmetric.NewMeterProvider())
	global.SetLoggerProvider(sdklog.NewLoggerProvider())

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		return nil
	}
	return shutdown, nil
}
