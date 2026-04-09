package mcpserver

import (
	"log/slog"
	"net/http"

	mcptransport "github.com/mark3labs/mcp-go/server"
)

// HTTPOptions configures how the MCP HTTP transports are exposed.
type HTTPOptions struct {
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

	// StreamableHTTPEndpoint is the path for the StreamableHTTP transport.
	// Default: /mcp
	StreamableHTTPEndpoint string
}

// SSEOptions configures how the SSE transport is exposed.
type SSEOptions = HTTPOptions

// NewHTTPHandler constructs a shared MCP server and returns an http.Handler
// that serves the StreamableHTTP and SSE transport endpoints plus a health
// check.
//
// The handler exposes:
//   - GET    /mcp/sse       — SSE connection endpoint
//   - POST   /mcp/messages  — SSE JSON-RPC message endpoint
//   - GET    /mcp           — StreamableHTTP listen endpoint
//   - POST   /mcp           — StreamableHTTP JSON-RPC endpoint
//   - DELETE /mcp           — StreamableHTTP session termination endpoint
//   - GET    /healthz       — health check
func NewHTTPHandler(logger *slog.Logger, client SearchClient, opts HTTPOptions) http.Handler {
	if opts.SSEEndpoint == "" {
		opts.SSEEndpoint = "/mcp/sse"
	}
	if opts.MessageEndpoint == "" {
		opts.MessageEndpoint = "/mcp/messages"
	}
	if opts.StreamableHTTPEndpoint == "" {
		opts.StreamableHTTPEndpoint = "/mcp"
	}

	mcpSrv := New(logger, client)

	sseOpts := []mcptransport.SSEOption{
		mcptransport.WithSSEEndpoint(opts.SSEEndpoint),
		mcptransport.WithMessageEndpoint(opts.MessageEndpoint),
	}
	if opts.BaseURL != "" {
		sseOpts = append(sseOpts, mcptransport.WithBaseURL(opts.BaseURL))
	}

	sseServer := mcptransport.NewSSEServer(mcpSrv, sseOpts...)
	streamableHTTPServer := mcptransport.NewStreamableHTTPServer(mcpSrv)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle(opts.SSEEndpoint, sseServer)
	mux.Handle(opts.MessageEndpoint, sseServer)
	mux.Handle(opts.StreamableHTTPEndpoint, streamableHTTPServer)

	return mux
}

// NewSSEHandler constructs an MCP HTTP handler with the SSE transport exposed.
// It is kept as a compatibility wrapper for existing callers.
func NewSSEHandler(logger *slog.Logger, client SearchClient, opts SSEOptions) http.Handler {
	return NewHTTPHandler(logger, client, HTTPOptions(opts))
}
