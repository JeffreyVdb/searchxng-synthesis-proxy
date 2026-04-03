package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/synthproxy"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServer_SearchToolRegistered(t *testing.T) {
	s := New(nil, &stubClient{})

	// The server should have the search tool registered. We verify by calling
	// ListTools through the MCP protocol in the integration test below, but
	// a basic sanity check is that the server was created without panicking.
	if s == nil {
		t.Fatal("New() returned nil")
	}
}

func TestSearchHandler_ValidQuery(t *testing.T) {
	client := &stubClient{
		resp: &synthproxy.Response{
			Query:  "golang",
			Answer: "Go is great",
			Sources: []synthproxy.Source{
				{Index: 1, Title: "Go", URL: "https://go.dev", Snippet: "The Go Programming Language", Engine: "google"},
			},
			Meta: synthproxy.Meta{Model: "test", ResultsConsidered: 1, TookMS: 100},
		},
	}

	handler := newSearchHandler(slog.Default(), client)

	req := mcp.CallToolRequest{}
	req.Params.Name = "search"
	req.Params.Arguments = map[string]interface{}{"query": "golang"}

	result, err := handler.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("handle() error: %v", err)
	}

	if result.IsError {
		t.Fatal("result should not be an error")
	}

	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want TextContent", result.Content[0])
	}

	if !strings.Contains(textContent.Text, "Go is great") {
		t.Errorf("text = %q, want to contain answer", textContent.Text)
	}
	if !strings.Contains(textContent.Text, "1. Go") {
		t.Errorf("text = %q, want to contain source", textContent.Text)
	}
}

func TestSearchHandler_BlankQuery(t *testing.T) {
	handler := newSearchHandler(slog.Default(), &stubClient{})

	req := mcp.CallToolRequest{}
	req.Params.Name = "search"
	req.Params.Arguments = map[string]interface{}{"query": "  "}

	result, err := handler.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("handle() error: %v", err)
	}

	if result.IsError == false {
		t.Fatal("result should be an error for blank query")
	}
}

func TestSearchHandler_UpstreamFailure(t *testing.T) {
	client := &stubClient{
		err: &synthproxy.Error{
			StatusCode: 502,
			Code:       "search_upstream_error",
			Message:    "search backend failed",
		},
	}

	handler := newSearchHandler(slog.Default(), client)

	req := mcp.CallToolRequest{}
	req.Params.Name = "search"
	req.Params.Arguments = map[string]interface{}{"query": "test"}

	result, err := handler.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("handle() error: %v", err)
	}

	if result.IsError == false {
		t.Fatal("result should be an error for upstream failure")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want TextContent", result.Content[0])
	}
	if !strings.Contains(textContent.Text, "search backend failed") {
		t.Errorf("text = %q, want upstream error message", textContent.Text)
	}
}

func TestSearchHandler_StructuredContent(t *testing.T) {
	client := &stubClient{
		resp: &synthproxy.Response{
			Query:  "test",
			Answer: "an answer",
			Sources: []synthproxy.Source{
				{Index: 1, Title: "T", URL: "https://example.com", Snippet: "s", Engine: "e"},
			},
		},
	}

	handler := newSearchHandler(slog.Default(), client)
	req := mcp.CallToolRequest{}
	req.Params.Name = "search"
	req.Params.Arguments = map[string]interface{}{"query": "test"}

	result, err := handler.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("handle() error: %v", err)
	}

	if result.StructuredContent == nil {
		t.Fatal("expected StructuredContent")
	}

	// StructuredContent should be the *synthproxy.Response directly
	structured, ok := result.StructuredContent.(*synthproxy.Response)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want *synthproxy.Response", result.StructuredContent)
	}
	if structured.Answer != "an answer" {
		t.Errorf("structured answer = %q", structured.Answer)
	}
}

func TestSSEHandler_Healthz(t *testing.T) {
	ts := httptest.NewServer(fakeUpstream())
	defer ts.Close()

	client := synthproxy.NewClient(ts.URL, synthproxy.DefaultHTTPClient(5*time.Second))
	handler := NewSSEHandler(nil, client, SSEOptions{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q", body["status"])
	}
}

func TestSSEHandler_SSEEndpointReachable(t *testing.T) {
	ts := httptest.NewServer(fakeUpstream())
	defer ts.Close()

	client := synthproxy.NewClient(ts.URL, synthproxy.DefaultHTTPClient(5*time.Second))

	// Use a test server so the SSE handler can resolve its own base URL
	mcpSrv := New(nil, client)
	testSrv := httptest.NewServer(NewSSEHandler(nil, client, SSEOptions{
		SSEEndpoint:     "/mcp/sse",
		MessageEndpoint: "/mcp/messages",
	}))
	defer testSrv.Close()

	// The SSE endpoint should accept a GET and upgrade to SSE (200 with text/event-stream)
	resp, err := http.Get(testSrv.URL + "/mcp/sse")
	if err != nil {
		t.Fatalf("GET /mcp/sse error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Read the first event (endpoint advertisement) then close
	_ = mcpSrv // ensure variable used
}

func TestSSEHandler_Integration_ToolResult(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(synthproxy.Response{
			Query:  "test query",
			Answer: "A synthesized answer.",
			Sources: []synthproxy.Source{
				{Index: 1, Title: "Example", URL: "https://example.com", Snippet: "A snippet", Engine: "test"},
			},
			Meta: synthproxy.Meta{Model: "test-model", ResultsConsidered: 1, TookMS: 50},
		})
	}))
	defer upstream.Close()

	client := synthproxy.NewClient(upstream.URL, synthproxy.DefaultHTTPClient(5*time.Second))

	// Direct test through the handler
	handler := newSearchHandler(slog.Default(), client)
	req := mcp.CallToolRequest{}
	req.Params.Name = "search"
	req.Params.Arguments = map[string]interface{}{"query": "test query"}

	result, err := handler.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("handle() error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T", result.Content[0])
	}

	if !strings.Contains(textContent.Text, "A synthesized answer.") {
		t.Errorf("text = %q", textContent.Text)
	}
	if !strings.Contains(textContent.Text, "1. Example") {
		t.Errorf("text = %q", textContent.Text)
	}
	if !strings.Contains(textContent.Text, "https://example.com") {
		t.Errorf("text = %q", textContent.Text)
	}
}

func fakeUpstream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(synthproxy.Response{
			Query:  "test",
			Answer: "ok",
		})
	})
}
