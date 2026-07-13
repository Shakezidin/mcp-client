package mcpclient

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolClient is the interface external LLM integrations should use.
// It abstracts MCP session management, tool discovery, and tool invocation.
type ToolClient interface {
	Connect(ctx context.Context) error
	Close() error
	ListTools(ctx context.Context) ([]*ToolDescriptor, error)
	RefreshTools(ctx context.Context) ([]*ToolDescriptor, error)
	GetTool(ctx context.Context, name string) (*ToolDescriptor, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*ToolCallResult, error)
	CallToolInto(ctx context.Context, name string, args map[string]any, out any) (*ToolCallResult, error)
	CallToolJSON(ctx context.Context, name string, args map[string]any) ([]byte, error)
}

// ToolCallResult is the normalized result from a tool invocation.
type ToolCallResult struct {
	IsError           bool                `json:"isError"`
	Content           []mcp.Content       `json:"content,omitempty"`
	StructuredContent any                 `json:"structuredContent,omitempty"`
	RawResponse       *mcp.CallToolResult `json:"-"`
}

// ToolAnnotations exposes a subset of tool hints useful for LLM clients.
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint,omitempty"`
	IdempotentHint  bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
}

// ToolDescriptor is the normalized public metadata for a remote MCP tool.
type ToolDescriptor struct {
	Name         string           `json:"name"`
	Title        string           `json:"title,omitempty"`
	Description  string           `json:"description,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
	InputSchema  any              `json:"inputSchema,omitempty"`
	OutputSchema any              `json:"outputSchema,omitempty"`
	Meta         any              `json:"_meta,omitempty"`
}
