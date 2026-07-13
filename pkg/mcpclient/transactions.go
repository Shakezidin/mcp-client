package mcpclient

import (
	"context"
	"time"
)

// TransactionEvent describes a client-side operation for audit and observability.
type TransactionEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Operation string    `json:"operation"`
	ToolName  string    `json:"toolName,omitempty"`
	Arguments any       `json:"arguments,omitempty"`
	Result    any       `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// TransactionLogger receives transaction events for external logging systems.
type TransactionLogger interface {
	LogTransaction(ctx context.Context, event TransactionEvent) error
}
