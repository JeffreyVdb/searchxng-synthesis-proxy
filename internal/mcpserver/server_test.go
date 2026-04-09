package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/synthproxy"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
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
	// The sanitized message should NOT contain internal transport details.
	if strings.Contains(textContent.Text, "search backend failed") {
		t.Errorf("text = %q, should not contain raw upstream message", textContent.Text)
	}
	if !strings.Contains(textContent.Text, "search_upstream_error") {
		t.Errorf("text = %q, want sanitized message with error code", textContent.Text)
	}
}

func TestSearchHandler_TransportError(t *testing.T) {
	client := &stubClient{
		err: &synthproxy.TransportError{Cause: fmt.Errorf("dial tcp 10.0.0.5:8080: connection refused")},
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
		t.Fatal("result should be an error for transport failure")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want TextContent", result.Content[0])
	}
	// Must not leak internal IPs or transport details.
	if strings.Contains(textContent.Text, "10.0.0.5") {
		t.Errorf("text = %q, should not contain internal IP", textContent.Text)
	}
	if !strings.Contains(textContent.Text, "search upstream unavailable") {
		t.Errorf("text = %q, want sanitized transport message", textContent.Text)
	}
}

func TestSearchHandler_DecodeError(t *testing.T) {
	client := &stubClient{
		err: &synthproxy.DecodeError{Cause: fmt.Errorf("json: cannot unmarshal number into Go string")},
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
		t.Fatal("result should be an error for decode failure")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want TextContent", result.Content[0])
	}
	if strings.Contains(textContent.Text, "json:") {
		t.Errorf("text = %q, should not contain internal Go error", textContent.Text)
	}
	if !strings.Contains(textContent.Text, "search upstream returned an invalid response") {
		t.Errorf("text = %q, want sanitized decode message", textContent.Text)
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

func TestHTTPHandler_Healthz(t *testing.T) {
	ts := httptest.NewServer(fakeUpstream())
	defer ts.Close()

	client := synthproxy.NewClient(ts.URL, synthproxy.DefaultHTTPClient(5*time.Second))
	handler := NewHTTPHandler(nil, client, HTTPOptions{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
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

func TestHTTPHandler_SSEEndpointReachable(t *testing.T) {
	ts := httptest.NewServer(fakeUpstream())
	defer ts.Close()

	client := synthproxy.NewClient(ts.URL, synthproxy.DefaultHTTPClient(5*time.Second))

	testSrv := httptest.NewServer(NewHTTPHandler(nil, client, HTTPOptions{
		SSEEndpoint:            "/mcp/sse",
		MessageEndpoint:        "/mcp/messages",
		StreamableHTTPEndpoint: "/mcp",
	}))
	defer testSrv.Close()

	resp, err := http.Get(testSrv.URL + "/mcp/sse")
	if err != nil {
		t.Fatalf("GET /mcp/sse error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestHTTPHandler_StreamableHTTPInitialize(t *testing.T) {
	testSrv := newTestMCPHTTPServer(t, fakeUpstream())
	defer testSrv.Close()

	mcpClient := newStreamableHTTPClient(t, testSrv.URL+"/mcp")
	defer mcpClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := mcpClient.Initialize(ctx, initializeRequest())
	if err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	if result.ServerInfo.Name != "Search Synthesis Proxy" {
		t.Fatalf("server name = %q, want %q", result.ServerInfo.Name, "Search Synthesis Proxy")
	}
	if result.Capabilities.Tools == nil {
		t.Fatal("expected tools capability")
	}
}

func TestHTTPHandler_StreamableHTTPToolsListAndCall(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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

	testSrv := newTestMCPHTTPServer(t, upstream.Config.Handler)
	defer testSrv.Close()

	mcpClient := newStreamableHTTPClient(t, testSrv.URL+"/mcp")
	defer mcpClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := mcpClient.Initialize(ctx, initializeRequest()); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}
	if len(toolsResult.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	foundSearch := false
	for _, tool := range toolsResult.Tools {
		if tool.Name == "search" {
			foundSearch = true
			break
		}
	}
	if !foundSearch {
		t.Fatalf("tools/list missing search tool: %+v", toolsResult.Tools)
	}

	callResult, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search",
			Arguments: map[string]any{
				"query": "test query",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if callResult.IsError {
		t.Fatal("tool call returned error result")
	}
	if len(callResult.Content) == 0 {
		t.Fatal("tool call returned no content")
	}

	textContent, ok := callResult.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want TextContent", callResult.Content[0])
	}
	if !strings.Contains(textContent.Text, "A synthesized answer.") {
		t.Errorf("text = %q, want synthesized answer", textContent.Text)
	}
	if !strings.Contains(textContent.Text, "https://example.com") {
		t.Errorf("text = %q, want source URL", textContent.Text)
	}
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

func newTestMCPHTTPServer(t *testing.T, upstream http.Handler) *httptest.Server {
	t.Helper()

	upstreamServer := httptest.NewServer(upstream)
	t.Cleanup(upstreamServer.Close)

	client := synthproxy.NewClient(upstreamServer.URL, synthproxy.DefaultHTTPClient(5*time.Second))

	return httptest.NewServer(NewHTTPHandler(nil, client, HTTPOptions{}))
}

func newStreamableHTTPClient(t *testing.T, serverURL string) *mcpclient.Client {
	t.Helper()

	transport, err := mcptransport.NewStreamableHTTP(serverURL)
	if err != nil {
		t.Fatalf("NewStreamableHTTP() error: %v", err)
	}
	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	client := mcpclient.NewClient(transport)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("client.Start() error: %v", err)
	}

	return client
}

func initializeRequest() mcp.InitializeRequest {
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{
		Name:    "searchxng-synthesis-proxy-test-client",
		Version: "1.0.0",
	}
	request.Params.Capabilities = mcp.ClientCapabilities{}
	return request
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
