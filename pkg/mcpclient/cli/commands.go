package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/npire37/aib-mcp-client/pkg/mcpclient"
)

// RunConfig holds the command-mode execution configuration.
type RunConfig struct {
	Endpoint     string
	Tool         string
	ArgsJSON     string
	HeaderString string
	ListTools    bool
	Mode         string
	HTTPPort     int
}

// RunCLI executes the requested CLI mode.
func RunCLI(ctx context.Context, cfg RunConfig) error {
	switch cfg.Mode {
	case "http":
		return StartHTTPServer(ctx, cfg.Endpoint, cfg.HTTPPort)
	case "interactive":
		StartInteractiveClient(ctx, cfg.Endpoint)
		return nil
	case "cli":
		return runLocalCLI(ctx, cfg)
	default:
		return fmt.Errorf("unknown mode %q", cfg.Mode)
	}
}

func runLocalCLI(ctx context.Context, cfg RunConfig) error {
	client, err := newConnectedClient(ctx, cfg.Endpoint)
	if err != nil {
		return err
	}
	defer client.Close()

	if cfg.ListTools {
		return writeJSON(os.Stdout, map[string]any{"tools": mustListTools(ctx, client)})
	}

	if cfg.Tool == "" {
		return fmt.Errorf("either --list-tools or --tool must be provided")
	}

	args, err := parseArgsJSON(cfg.ArgsJSON)
	if err != nil {
		return err
	}

	headers, err := parseHeaderString(cfg.HeaderString)
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		if args == nil {
			args = make(map[string]any)
		}
		args["headers"] = headers
	}

	result, err := client.CallTool(ctx, cfg.Tool, args)
	if err != nil {
		return err
	}

	return writeJSON(os.Stdout, result)
}

func mustListTools(ctx context.Context, client *mcpclient.Client) []*mcpclient.ToolDescriptor {
	tools, err := client.ListTools(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list tools: %v\n", err)
		os.Exit(1)
	}
	return tools
}

func StartHTTPServer(ctx context.Context, endpoint string, port int) error {
	client, err := newConnectedClient(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			headers := w.Header()
			headers.Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tools, err := client.ListTools(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list tools: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"tools": tools})
	})

	mux.HandleFunc("/call", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			headers := w.Header()
			headers.Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Tool    string              `json:"tool"`
			Args    map[string]any      `json:"args,omitempty"`
			Headers map[string][]string `json:"headers,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		if req.Tool == "" {
			http.Error(w, "tool name is required", http.StatusBadRequest)
			return
		}
		if len(req.Headers) > 0 {
			if req.Args == nil {
				req.Args = make(map[string]any)
			}
			req.Args["headers"] = req.Headers
		}

		result, err := client.CallTool(ctx, req.Tool, req.Args)
		if err != nil {
			http.Error(w, fmt.Sprintf("tool invocation failed: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"result": result})
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Fprintf(os.Stdout, "MCP HTTP wrapper listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func parseArgsJSON(value string) (map[string]any, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(value), &args); err != nil {
		return nil, fmt.Errorf("invalid args JSON: %w", err)
	}
	return args, nil
}

func parseHeaderString(value string) (http.Header, error) {
	headers := make(http.Header)
	if strings.TrimSpace(value) == "" {
		return headers, nil
	}

	for _, pair := range strings.Split(value, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed header token %q: expected Name: Value", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("header key cannot be empty")
		}
		headers.Add(key, value)
	}

	return headers, nil
}

func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
