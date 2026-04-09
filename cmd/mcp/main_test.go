package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/config"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestRun_MissingConfig(t *testing.T) {
	// Ensure MCP_PROXY_BASE_URL is unset so config loading fails.
	orig := os.Getenv("MCP_PROXY_BASE_URL")
	os.Unsetenv("MCP_PROXY_BASE_URL")
	defer os.Setenv("MCP_PROXY_BASE_URL", orig)

	err := run()
	if err == nil {
		t.Fatal("expected error when MCP_PROXY_BASE_URL is not set")
	}
	if !strings.Contains(err.Error(), "MCP_PROXY_BASE_URL") {
		t.Errorf("error = %q, want mention of MCP_PROXY_BASE_URL", err.Error())
	}
}

func TestNewServer_ExposesHealthzAndStreamableHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"query":   "test",
			"answer":  "ok",
			"sources": []map[string]any{},
			"meta": map[string]any{
				"model":              "test",
				"results_considered": 0,
				"took_ms":            1,
			},
		})
	}))
	defer upstream.Close()

	cfg := config.MCPConfig{
		Port:            "8090",
		ProxyBaseURL:    upstream.URL,
		RequestTimeout:  5 * time.Second,
		ReadTimeout:     5 * time.Second,
		IdleTimeout:     5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	srv, err := newServer(cfg, slog.Default())
	if err != nil {
		t.Fatalf("newServer() error: %v", err)
	}

	testSrv := httptest.NewServer(srv.Handler)
	defer testSrv.Close()

	resp, err := http.Get(testSrv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", resp.StatusCode)
	}

	transport, err := mcptransport.NewStreamableHTTP(testSrv.URL + "/mcp")
	if err != nil {
		t.Fatalf("NewStreamableHTTP() error: %v", err)
	}
	defer transport.Close()
	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("transport.Start() error: %v", err)
	}

	client := mcpclient.NewClient(transport)
	defer client.Close()
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("client.Start() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "cmd-mcp-test", Version: "1.0.0"}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	result, err := client.Initialize(ctx, initReq)
	if err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}
	if result.ServerInfo.Name != "Search Synthesis Proxy" {
		t.Fatalf("server name = %q", result.ServerInfo.Name)
	}
}
