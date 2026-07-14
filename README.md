# aib-mcp-client

A reusable Go client wrapper for MCP (Model Context Protocol) servers.

## Project layout

- `pkg/mcpclient/` — reusable MCP client library
- `cmd/mcpclient/` — demo executable showing tool discovery and tool invocation

## Usage

### Build demo

```bash
cd /Users/npire37/Downloads/HDFC\ project/aib-mcp-client
make build-demo
```

### Build production CLI

```bash
cd /Users/npire37/Downloads/HDFC\ project/aib-mcp-client
make build-prod
```

### Run demo

```bash
export MCP_ENDPOINT=http://localhost:8080/ai-mb-mcp/v0beta/mcp
export OTEL_ENDPOINT=localhost
export OTEL_PORT=4317
make run
```

### Run production CLI

```bash
export MCP_ENDPOINT=http://localhost:8080/ai-mb-mcp/v0beta/mcp
export OTEL_ENDPOINT=localhost
export OTEL_PORT=4317
make run-prod
```

### Run tests

```bash
make test
```

### Format code

```bash
make fmt
```

### Cleanup

```bash
make clean
```

## Observability

The demo and production binaries both initialize OpenTelemetry using OTLP gRPC to the configured collector endpoint. The default is:

- `OTEL_ENDPOINT=localhost`
- `OTEL_PORT=4317`

If you run an OpenTelemetry collector locally, the client will export traces, metrics, and logs over OTLP gRPC.

## Package

Import the client from the library package:

```go
import "github.com/npire37/aib-mcp-client/pkg/mcpclient"
```

## Features

- MCP tool discovery
- Tool invocation by name
- HTTP header injection
- OpenTelemetry tracing integration
- Transaction logging hook for audit / PubSub


find_total_account_balance
{"hashUserId": "1234567890"}