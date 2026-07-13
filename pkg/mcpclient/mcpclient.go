package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// types and helpers split into separate files under this package.

// Client provides a reusable, industry-standard MCP wrapper.
type Client struct {
	cfg       Config
	client    *mcp.Client
	session   *mcp.ClientSession
	transport *mcp.StreamableClientTransport
	mu        sync.RWMutex
	toolCache []*ToolDescriptor
}

var _ ToolClient = (*Client)(nil)

// New creates a new reusable MCP client wrapper.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("mcpclient: Endpoint is required")
	}
	if cfg.ClientName == "" {
		cfg.ClientName = "aib-mcp-client"
	}
	if cfg.ClientVersion == "" {
		cfg.ClientVersion = "v1.0.0"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.ClientOptions == nil {
		cfg.ClientOptions = &mcp.ClientOptions{}
	}
	if cfg.ClientOptions.KeepAlive == 0 {
		cfg.ClientOptions.KeepAlive = 30 * time.Second
	}
	if cfg.KeepAlive > 0 {
		cfg.ClientOptions.KeepAlive = cfg.KeepAlive
	}

	return &Client{cfg: cfg}, nil
}

// Connect establishes an MCP session using the configured endpoint.
func (c *Client) tracer() trace.Tracer {
	if c.cfg.TracerProvider != nil {
		return c.cfg.TracerProvider.Tracer("mcpclient")
	}
	return otel.Tracer("mcpclient")
}

func (c *Client) logger() *slog.Logger {
	return c.cfg.Logger
}

func (c *Client) logf(msg string, kv ...any) {
	if logger := c.logger(); logger != nil {
		logger.Info(msg, kv...)
	}
}

func (c *Client) logTransactionEvent(ctx context.Context, event TransactionEvent) {
	if c.cfg.TransactionLogger == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if err := c.cfg.TransactionLogger.LogTransaction(ctx, event); err != nil {
		c.logf("transaction logger failed", "error", err, "tool", event.ToolName)
	}
}

func (c *Client) SetHTTPHeaders(headers http.Header) {
	if headers == nil {
		c.cfg.HTTPHeaders = nil
		return
	}
	c.cfg.HTTPHeaders = headers.Clone()
}

func (c *Client) Connect(ctx context.Context) error {
	if c.session != nil {
		return nil
	}

	ctx, span := c.tracer().Start(ctx, "mcpclient.Connect")
	defer span.End()

	if c.cfg.HTTPClient == nil {
		c.cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if len(c.cfg.HTTPHeaders) > 0 {
		transport := c.cfg.HTTPClient.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		c.cfg.HTTPClient.Transport = &headerRoundTripper{
			base:    transport,
			headers: c.cfg.HTTPHeaders.Clone(),
		}
	}

	c.logf("connecting to MCP endpoint", "endpoint", c.cfg.Endpoint)

	c.client = mcp.NewClient(&mcp.Implementation{
		Name:    c.cfg.ClientName,
		Version: c.cfg.ClientVersion,
	}, c.cfg.ClientOptions)

	c.transport = &mcp.StreamableClientTransport{
		Endpoint:             c.cfg.Endpoint,
		HTTPClient:           c.cfg.HTTPClient,
		DisableStandaloneSSE: c.cfg.DisableStandaloneSSE,
	}

	session, err := c.client.Connect(ctx, c.transport, nil)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("connect mcp session: %w", err)
	}

	c.session = session
	c.logTransactionEvent(ctx, TransactionEvent{
		Operation: "connect",
	})
	return nil
}

// Close shuts down the current MCP session.
func (c *Client) Close() error {
	if c.session == nil {
		return nil
	}

	err := c.session.Close()
	c.session = nil
	c.mu.Lock()
	c.toolCache = nil
	c.mu.Unlock()
	return err
}

func (c *Client) ensureConnected() error {
	if c.session == nil {
		return errors.New("mcpclient: session is not connected")
	}
	return nil
}

// RefreshTools fetches the latest tool metadata and updates the cache.
func (c *Client) RefreshTools(ctx context.Context) ([]*ToolDescriptor, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}
	ctx, span := c.tracer().Start(ctx, "mcpclient.RefreshTools")
	defer span.End()

	result, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("refresh tools: %w", err)
	}

	descriptors := make([]*ToolDescriptor, 0, len(result.Tools))
	for _, tool := range result.Tools {
		descriptors = append(descriptors, toolToDescriptor(tool))
	}

	c.mu.Lock()
	c.toolCache = descriptors
	c.mu.Unlock()

	return descriptors, nil
}

// ListTools returns cached tool metadata or refreshes it from the server.
func (c *Client) ListTools(ctx context.Context) ([]*ToolDescriptor, error) {
	c.mu.RLock()
	cached := c.toolCache
	c.mu.RUnlock()

	if len(cached) > 0 {
		copyList := make([]*ToolDescriptor, len(cached))
		copy(copyList, cached)
		return copyList, nil
	}

	return c.RefreshTools(ctx)
}

// GetTool returns a tool descriptor by name.
func (c *Client) GetTool(ctx context.Context, name string) (*ToolDescriptor, error) {
	if name == "" {
		return nil, errors.New("tool name cannot be empty")
	}

	tools, err := c.ListTools(ctx)
	if err != nil {
		return nil, err
	}

	for _, tool := range tools {
		if tool.Name == name {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

// CallTool invokes a named tool and returns normalized result data.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*ToolCallResult, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}
	ctx, span := c.tracer().Start(ctx, "mcpclient.CallTool")
	defer span.End()

	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		span.RecordError(err)
		c.logTransactionEvent(ctx, TransactionEvent{
			Operation: "call_tool",
			ToolName:  name,
			Arguments: args,
			Error:     err.Error(),
		})
		return nil, fmt.Errorf("call tool %q: %w", name, err)
	}

	toolResult := &ToolCallResult{
		IsError:           result.IsError,
		Content:           result.Content,
		StructuredContent: result.StructuredContent,
		RawResponse:       result,
	}
	c.logTransactionEvent(ctx, TransactionEvent{
		Operation: "call_tool",
		ToolName:  name,
		Arguments: args,
		Result:    toolResult,
	})
	return toolResult, nil
}

// CallToolInto invokes a tool and unmarshals structured output into the provided destination.
func (c *Client) CallToolInto(ctx context.Context, name string, args map[string]any, out any) (*ToolCallResult, error) {
	res, err := c.CallTool(ctx, name, args)
	if err != nil {
		return nil, err
	}
	if res.StructuredContent == nil || out == nil {
		return res, nil
	}

	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("marshal structured output: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("unmarshal structured output: %w", err)
	}

	return res, nil
}

// CallToolJSON invokes a tool and returns the raw JSON response.
func (c *Client) CallToolJSON(ctx context.Context, name string, args map[string]any) ([]byte, error) {
	res, err := c.CallTool(ctx, name, args)
	if err != nil {
		return nil, err
	}
	return json.Marshal(res.RawResponse)
}

func toolToDescriptor(tool *mcp.Tool) *ToolDescriptor {
	annotations := &ToolAnnotations{}
	if tool.Annotations != nil {
		annotations = &ToolAnnotations{
			Title:           tool.Annotations.Title,
			ReadOnlyHint:    tool.Annotations.ReadOnlyHint,
			IdempotentHint:  tool.Annotations.IdempotentHint,
			OpenWorldHint:   tool.Annotations.OpenWorldHint,
			DestructiveHint: tool.Annotations.DestructiveHint,
		}
	}

	return &ToolDescriptor{
		Name:         tool.Name,
		Title:        tool.Title,
		Description:  tool.Description,
		Annotations:  annotations,
		InputSchema:  tool.InputSchema,
		OutputSchema: tool.OutputSchema,
		Meta:         tool.Meta,
	}
}
