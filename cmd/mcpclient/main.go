package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/npire37/aib-mcp-client/pkg/mcpclient"
)

func main() {
	ctx := context.Background()
	endpoint := os.Getenv("MCP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080/ai-mb-mcp/v0beta/mcp"
	}

	client, err := mcpclient.New(mcpclient.Config{
		Endpoint:      endpoint,
		ClientName:    "aib-mcp-client-production",
		ClientVersion: "v1.0.0",
		KeepAlive:     30 * time.Second,
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
	})
	if err != nil {
		log.Fatalf("failed to create MCP client: %v", err)
	}

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("failed to connect to MCP server: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close MCP client: %v", err)
		}
	}()

	tools, err := client.ListTools(ctx)
	if err != nil {
		log.Fatalf("failed to list tools: %v", err)
	}

	fmt.Printf("connected to MCP endpoint %s\n", endpoint)
	fmt.Printf("found %d tools:\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\nEnter a tool name to invoke, or press Enter to refresh tool list:")
		toolName, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("failed to read tool name: %v", err)
		}
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			fmt.Println("Refreshing tool list...")
			tools, err = client.RefreshTools(ctx)
			if err != nil {
				log.Printf("failed to refresh tools: %v", err)
				continue
			}
			fmt.Printf("found %d tools:\n", len(tools))
			for _, tool := range tools {
				fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
			}
			continue
		}

		option, err := prompt(reader, fmt.Sprintf("Enter JSON arguments for %q", toolName))
		if err != nil {
			log.Fatalf("failed to read tool arguments: %v", err)
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(option), &args); err != nil {
			fmt.Printf("invalid JSON: %v\n", err)
			continue
		}

		result, err := client.CallTool(ctx, toolName, args)
		if err != nil {
			fmt.Printf("tool call failed: %v\n", err)
			continue
		}

		if result.IsError {
			fmt.Println("tool returned an error")
		} else {
			fmt.Println("tool call succeeded")
		}
		printResult(result)
	}
}

func prompt(reader *bufio.Reader, message string) (string, error) {
	fmt.Printf("%s (JSON): ", message)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func printResult(result *mcpclient.ToolCallResult) {
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
