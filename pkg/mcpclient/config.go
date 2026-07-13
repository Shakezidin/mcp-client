package mcpclient

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

// Config holds the reusable MCP client configuration.
type Config struct {
	Endpoint             string
	ClientName           string
	ClientVersion        string
	KeepAlive            time.Duration
	DisableStandaloneSSE bool
	HTTPClient           *http.Client
	HTTPHeaders          http.Header
	Logger               *slog.Logger
	TracerProvider       trace.TracerProvider
	TransactionLogger    TransactionLogger
	ClientOptions        *mcp.ClientOptions
}
