// Package mcpserver constructs the MCP server, registers tools, and exposes
// the SSE transport HTTP handlers. Tool registration and search-client logic
// are kept independent from the transport wiring so the project can later add
// a streamable-HTTP endpoint without rewriting tool logic.
package mcpserver

import (
	"context"
	"log/slog"
	"strings"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/synthproxy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SearchClient is the interface for calling the upstream proxy.
type SearchClient interface {
	Search(ctx context.Context, query string) (*synthproxy.Response, error)
}

// New creates an MCP server with the search tool registered.
func New(logger *slog.Logger, client SearchClient) *server.MCPServer {
	if logger == nil {
		logger = slog.Default()
	}

	s := server.NewMCPServer(
		"Search Synthesis Proxy",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	tool := mcp.NewTool("search",
		mcp.WithDescription("Search the web through the synthesis proxy and return a synthesized answer with cited sources."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query string."),
		),
	)

	handler := newSearchHandler(logger, client)
	s.AddTool(tool, handler.handle)

	return s
}

type searchHandler struct {
	logger *slog.Logger
	client SearchClient
}

func newSearchHandler(logger *slog.Logger, client SearchClient) *searchHandler {
	return &searchHandler{logger: logger, client: client}
}

func (h *searchHandler) handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return mcp.NewToolResultError("query must not be empty"), nil
	}

	resp, err := h.client.Search(ctx, query)
	if err != nil {
		h.logger.Error("upstream search failed",
			slog.String("query", query),
			slog.String("error", err.Error()),
		)
		return mcp.NewToolResultError(sanitizeError(err)), nil
	}

	text := FormatResponse(resp)

	result := mcp.NewToolResultText(text)

	// Attach structured content so clients that support it can inspect the
	// machine-readable payload alongside the human-readable text.
	result.StructuredContent = resp

	return result, nil
}

// sanitizeError returns a safe, user-facing message from an upstream error.
// It never exposes internal transport details (hostnames, URLs, dial errors).
// The full error is already logged by the caller.
func sanitizeError(err error) string {
	// Check for typed upstream errors that implement UserMessage().
	switch e := err.(type) {
	case interface{ UserMessage() string }:
		return e.UserMessage()
	default:
		return "search failed"
	}
}
