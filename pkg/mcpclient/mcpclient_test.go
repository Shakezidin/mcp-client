package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockTransactionLogger struct {
	events []TransactionEvent
}

func (m *mockTransactionLogger) LogTransaction(_ context.Context, event TransactionEvent) error {
	m.events = append(m.events, event)
	return nil
}

func TestClient_AddHeaders_And_ListTools(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "hello", Description: "say hello", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}}, nil, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	client, err := New(Config{
		Endpoint:      httpServer.URL,
		ClientName:    "test-client",
		ClientVersion: "v1.0.0",
		HTTPHeaders:   http.Header{"X-Test-Header": {"test-value"}},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	if len(tools) != 1 || tools[0].Name != "hello" {
		t.Fatalf("expected tool hello, got %+v", tools)
	}
}

func TestClient_CallToolJSON_And_TransactionLogging(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo args", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, args, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	logger := &mockTransactionLogger{}
	client, err := New(Config{
		Endpoint:          httpServer.URL,
		ClientName:        "test-client",
		ClientVersion:     "v1.0.0",
		TransactionLogger: logger,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	_, err = client.CallToolJSON(context.Background(), "echo", map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("CallToolJSON failed: %v", err)
	}

	if len(logger.events) == 0 {
		t.Fatalf("expected transaction events to be logged")
	}
}
