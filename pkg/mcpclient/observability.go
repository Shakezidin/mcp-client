package mcpclient

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	log "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
)

var (
	metricsOnce            sync.Once
	toolCallCounter        metric.Int64Counter
	toolErrorCounter       metric.Int64Counter
	toolCallDuration       metric.Float64Histogram
	connectionAttempts     metric.Int64Counter
	connectionErrorCounter metric.Int64Counter
)

func initObservability() {
	metricsOnce.Do(func() {
		meter := otel.Meter("github.com/npire37/aib-mcp-client")

		toolCallCounter, _ = meter.Int64Counter(
			"mcpclient.tool.calls",
			metric.WithDescription("Total MCP tool call attempts"),
		)
		toolErrorCounter, _ = meter.Int64Counter(
			"mcpclient.tool.errors",
			metric.WithDescription("Total MCP tool call failures"),
		)
		toolCallDuration, _ = meter.Float64Histogram(
			"mcpclient.tool.call.duration",
			metric.WithDescription("MCP tool call duration in seconds"),
		)
		connectionAttempts, _ = meter.Int64Counter(
			"mcpclient.connection.attempts",
			metric.WithDescription("Total MCP connection attempts"),
		)
		connectionErrorCounter, _ = meter.Int64Counter(
			"mcpclient.connection.errors",
			metric.WithDescription("Total MCP connection failures"),
		)
	})
}

func emitLog(ctx context.Context, level log.Severity, message string, err error, attrs ...log.KeyValue) {
	logger := logglobal.Logger("github.com/npire37/aib-mcp-client")
	record := log.Record{}
	record.SetTimestamp(time.Now())
	record.SetSeverity(level)
	record.SetSeverityText(level.String())
	record.SetBody(log.StringValue(message))
	if err != nil {
		record.SetErr(err)
	}
	if len(attrs) > 0 {
		record.AddAttributes(attrs...)
	}
	logger.Emit(ctx, record)
}

func emitInfo(ctx context.Context, message string, attrs ...log.KeyValue) {
	emitLog(ctx, log.SeverityInfo, message, nil, attrs...)
}

func emitError(ctx context.Context, message string, err error, attrs ...log.KeyValue) {
	emitLog(ctx, log.SeverityError, message, err, attrs...)
}

func recordConnectionAttempt(ctx context.Context, endpoint string) {
	initObservability()
	connectionAttempts.Add(ctx, 1, metric.WithAttributes(
		attribute.String("mcp.endpoint", endpoint),
	))
}

func recordConnectionError(ctx context.Context, endpoint string) {
	initObservability()
	connectionErrorCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("mcp.endpoint", endpoint),
	))
}

func recordToolCall(ctx context.Context, toolName string, status string, duration time.Duration, err error) {
	initObservability()
	attributes := []attribute.KeyValue{
		attribute.String("mcp.tool.name", toolName),
		attribute.String("mcp.tool.status", status),
	}
	toolCallCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
	if err != nil {
		toolErrorCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
	}
	toolCallDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
}
