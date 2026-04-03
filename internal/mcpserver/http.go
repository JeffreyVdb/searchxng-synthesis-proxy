package mcpserver

import (
	"log/slog"
	"net/http"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/synthproxy"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// SSEOptions configures how the SSE transport is exposed.
type SSEOptions struct {
	// BaseURL is the externally reachable URL for the MCP server (optional).
	// If set, it is passed to the SSE server so it can advertise the correct
	// message endpoint to clients.
	BaseURL string

	// SSEEndpoint is the path for the SSE connection endpoint.
	// Default: /mcp/sse
	SSEEndpoint string

	// MessageEndpoint is the path for the POST message endpoint.
	// Default: /mcp/messages
	MessageEndpoint string
}

// NewSSEHandler constructs an MCP server and returns an http.Handler that
// serves both the SSE transport endpoints and a health check.
//
// The handler exposes:
//   - GET  /mcp/sse       — SSE connection endpoint
//   - POST /mcp/messages  — JSON-RPC message endpoint
//   - GET  /healthz       — health check
func NewSSEHandler(logger *slog.Logger, client *synthproxy.Client, opts SSEOptions) http.Handler {
	if opts.SSEEndpoint == "" {
		opts.SSEEndpoint = "/mcp/sse"
	}
	if opts.MessageEndpoint == "" {
		opts.MessageEndpoint = "/mcp/messages"
	}

	mcpSrv := New(logger, client)

	sseOpts := []mcpserver.SSEOption{
		mcpserver.WithSSEEndpoint(opts.SSEEndpoint),
		mcpserver.WithMessageEndpoint(opts.MessageEndpoint),
	}
	if opts.BaseURL != "" {
		sseOpts = append(sseOpts, mcpserver.WithBaseURL(opts.BaseURL))
	}

	sseServer := mcpserver.NewSSEServer(mcpSrv, sseOpts...)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", sseServer)

	return mux
}
