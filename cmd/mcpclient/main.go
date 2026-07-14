package main

import (
	"context"
	"fmt"
	"os"

	"github.com/npire37/aib-mcp-client/pkg/mcpclient/cli"
	"github.com/npire37/aib-mcp-client/pkg/observability"
)

func main() {
	ctx := context.Background()
	endpoint := os.Getenv("MCP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080/ai-mb-mcp/v0beta/mcp"
	}

	otelEndpoint := os.Getenv("OTEL_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "localhost"
	}

	otlpPort := 4317
	if port := os.Getenv("OTEL_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &otlpPort)
	}

	shutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName:    "aib-mcp-client",
		ServiceVersion: "v1.0.0",
		Environment:    "local",
		Endpoint:       otelEndpoint,
		Insecure:       true,
		OTLPPort:       otlpPort,
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			panic(err)
		}
	}()

	cli.StartInteractiveClient(ctx, endpoint)
}
