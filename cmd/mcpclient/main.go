package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/npire37/aib-mcp-client/pkg/mcpclient/cli"
	"github.com/npire37/aib-mcp-client/pkg/observability"
)

func main() {
	ctx := context.Background()
	defaultEndpoint := envOr("MCP_ENDPOINT", "http://localhost:8080/ai-mb-mcp/v0beta/mcp")
	defaultOtelEndpoint := envOr("OTEL_ENDPOINT", "localhost")
	defaultOtlpPort := envInt("OTEL_PORT", 4317)

	endpoint := flag.String("endpoint", defaultEndpoint, "MCP endpoint URL")
	mode := flag.String("mode", "http", "execution mode: cli|http|interactive")
	tool := flag.String("tool", "", "tool name to invoke")
	argsJSON := flag.String("args", "", "JSON arguments for the tool")
	headers := flag.String("headers", "", "comma-separated headers as Name: Value pairs")
	listTools := flag.Bool("list-tools", false, "list available MCP tools")
	httpPort := flag.Int("http-port", 8081, "HTTP server port when mode=http")
	otelEndpoint := flag.String("otel-endpoint", defaultOtelEndpoint, "OTLP collector endpoint")
	otlpPort := flag.Int("otel-port", defaultOtlpPort, "OTLP collector port")

	flag.Parse()

	shutdown, err := observability.Setup(ctx, observability.Config{
		ServiceName:    "aib-mcp-client",
		ServiceVersion: "v1.0.0",
		Environment:    "local",
		Endpoint:       *otelEndpoint,
		Insecure:       true,
		OTLPPort:       *otlpPort,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize observability: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "failed to shutdown observability: %v\n", err)
		}
	}()

	runCfg := cli.RunConfig{
		Endpoint:     *endpoint,
		Mode:         *mode,
		Tool:         *tool,
		ArgsJSON:     *argsJSON,
		HeaderString: *headers,
		ListTools:    *listTools,
		HTTPPort:     *httpPort,
	}

	fmt.Println("trace log")
	if err := cli.RunCLI(ctx, runCfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func envOr(name, defaultValue string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return defaultValue
}

func envInt(name string, defaultValue int) int {
	if value := os.Getenv(name); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
