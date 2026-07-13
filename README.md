# aib-mcp-client

A reusable Go client wrapper for MCP (Model Context Protocol) servers.

## Project layout

- `pkg/mcpclient/` — reusable MCP client library
- `cmd/mcpclient/` — demo executable showing tool discovery and tool invocation

## Usage

### Build demo

```bash
cd /Users/npire37/Downloads/HDFC\ project/aib-mcp-client
go build -o bin/mcpclient ./cmd/mcpclient
```

### Run demo

```bash
export MCP_ENDPOINT=http://localhost:8080/ai-mb-mcp/v0beta/mcp
./bin/mcpclient
```

### Run tests

```bash
go test ./...
```

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
