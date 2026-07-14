package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/npire37/aib-mcp-client/pkg/mcpclient"
	logapi "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
)

// StartInteractiveClient runs an interactive REPL for invoking MCP tools.
// It is the refactored, clearer entry point for interactive usage.
func StartInteractiveClient(ctx context.Context, endpoint string) {
	client, err := newConnectedClient(ctx, endpoint)
	if err != nil {
		emitCliError(ctx, "failed to start MCP client", err, endpoint)
		log.Fatalf("failed to start MCP client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			emitCliError(ctx, "failed to close MCP client", err, endpoint)
			log.Printf("failed to close MCP client: %v", err)
		}
	}()

	if err := printToolList(ctx, client, endpoint); err != nil {
		emitCliError(ctx, "failed to list tools", err, endpoint)
		log.Fatalf("failed to list tools: %v", err)
	}

	if err := interactiveREPL(ctx, client, endpoint); err != nil {
		emitCliError(ctx, "interactive loop failed", err, endpoint)
		log.Fatalf("interactive loop failed: %v", err)
	}
}

// newConnectedClient creates and connects an MCP client using sensible defaults.
func newConnectedClient(ctx context.Context, endpoint string) (*mcpclient.Client, error) {
	client, err := mcpclient.New(mcpclient.Config{
		Endpoint:      endpoint,
		ClientName:    "aib-mcp-client-production",
		ClientVersion: "v1.0.0",
		KeepAlive:     30 * time.Second,
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect client: %w", err)
	}
	return client, nil
}

// printToolList fetches tools and prints a brief summary to stdout.
func printToolList(ctx context.Context, client *mcpclient.Client, endpoint string) error {
	tools, err := client.ListTools(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("connected to MCP endpoint %s\n", endpoint)
	fmt.Printf("found %d tools:\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}
	return nil
}

// interactiveREPL runs a simple read-eval-print loop for invoking tools.
func interactiveREPL(ctx context.Context, client *mcpclient.Client, endpoint string) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		toolName, err := readToolName(reader)
		if err != nil {
			return fmt.Errorf("read tool name: %w", err)
		}
		if toolName == "" {
			// refresh and print
			if err := refreshAndPrint(client, ctx); err != nil {
				log.Printf("failed to refresh tools: %v", err)
			}
			continue
		}

		args, err := readJSONArgs(reader, toolName)
		if err != nil {
			emitCliError(ctx, "invalid tool arguments", err, endpoint, logapi.String("tool.name", toolName))
			log.Printf("invalid args: %v", err)
			continue
		}

		headers, err := readHeaders(reader)
		if err != nil {
			emitCliError(ctx, "failed to read headers", err, endpoint, logapi.String("tool.name", toolName))
			log.Printf("failed to read headers: %v", err)
			continue
		}

		// allow callers to pass headers inside the tool arguments if needed
		if args == nil {
			args = map[string]any{}
		}
		args["headers"] = headers

		if err := callToolAndPrint(ctx, client, toolName, args); err != nil {
			emitCliError(ctx, "tool call failed", err, endpoint, logapi.String("tool.name", toolName))
			log.Printf("tool call failed: %v", err)
		}
	}
}

func readToolName(reader *bufio.Reader) (string, error) {
	fmt.Println("\nEnter a tool name to invoke, or press Enter to refresh tool list:")
	toolName, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(toolName), nil
}

func readJSONArgs(reader *bufio.Reader, toolName string) (map[string]any, error) {
	fmt.Printf("Enter JSON arguments for %q (single-line): ", toolName)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(line), &args); err != nil {
		return nil, err
	}
	return args, nil
}

func readHeaders(reader *bufio.Reader) (http.Header, error) {
	fmt.Println("Enter headers as comma-separated Name: Value pairs, or press Enter to skip:")
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	return parseHeaders(strings.TrimSpace(line))
}

func parseHeaders(val string) (http.Header, error) {
	headers := make(http.Header)

	if val == "" {
		return headers, nil
	}

	// Split individual headers by comma
	pairs := strings.Split(val, ",")
	for _, pair := range pairs {
		// Split into Key and Value components
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("malformed header token missing a colon")
		}

		// Sanitize whitespaces around headers
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, errors.New("empty header key found")
		}

		headers.Add(key, value)
	}

	return headers, nil
}

func refreshAndPrint(client *mcpclient.Client, ctx context.Context) error {
	tools, err := client.RefreshTools(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("found %d tools:\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}
	return nil
}

func callToolAndPrint(ctx context.Context, client *mcpclient.Client, toolName string, args map[string]any) error {
	result, err := client.CallTool(ctx, toolName, args)
	if err != nil {
		return err
	}
	if result.IsError {
		emitCliError(ctx, "tool returned error result", nil, "", logapi.String("tool.name", toolName))
		fmt.Println("tool returned an error")
	} else {
		emitCliInfo(ctx, "tool call succeeded", logapi.String("tool.name", toolName))
		fmt.Println("tool call succeeded")
	}
	printToolResult(result)
	return nil
}

func printToolResult(result *mcpclient.ToolCallResult) {
	if len(result.Content) > 0 {
		fmt.Println("content:")
		for _, content := range result.Content {
			switch v := content.(type) {
			case *mcp.TextContent:
				fmt.Printf("- text: %s\n", v.Text)
			default:
				fmt.Printf("- content: %#v\n", content)
			}
		}
	}
	if result.StructuredContent != nil {
		fmt.Printf("structured content: %#v\n", result.StructuredContent)
	}
}

func cliLogger() logapi.Logger {
	return logglobal.Logger("github.com/npire37/aib-mcp-client/cli")
}

func emitCliInfo(ctx context.Context, message string, attrs ...logapi.KeyValue) {
	logger := cliLogger()
	record := logapi.Record{}
	record.SetTimestamp(time.Now())
	record.SetSeverity(logapi.SeverityInfo)
	record.SetSeverityText(logapi.SeverityInfo.String())
	record.SetBody(logapi.StringValue(message))
	if len(attrs) > 0 {
		record.AddAttributes(attrs...)
	}
	logger.Emit(ctx, record)
}

func emitCliError(ctx context.Context, message string, err error, endpoint string, attrs ...logapi.KeyValue) {
	logger := cliLogger()
	record := logapi.Record{}
	record.SetTimestamp(time.Now())
	record.SetSeverity(logapi.SeverityError)
	record.SetSeverityText(logapi.SeverityError.String())
	record.SetBody(logapi.StringValue(message))
	if err != nil {
		record.SetErr(err)
	}
	if endpoint != "" {
		record.AddAttributes(logapi.String("mcp.endpoint", endpoint))
	}
	if len(attrs) > 0 {
		record.AddAttributes(attrs...)
	}
	logger.Emit(ctx, record)
}
