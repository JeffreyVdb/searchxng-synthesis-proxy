package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadMCP_Defaults(t *testing.T) {
	t.Setenv("MCP_PROXY_BASE_URL", "http://127.0.0.1:8080")

	cfg, err := LoadMCP()
	if err != nil {
		t.Fatalf("LoadMCP() error: %v", err)
	}

	if cfg.ProxyBaseURL != "http://127.0.0.1:8080" {
		t.Errorf("ProxyBaseURL = %q, want %q", cfg.ProxyBaseURL, "http://127.0.0.1:8080")
	}
	if cfg.Port != "8090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8090")
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 30*time.Second)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, 10*time.Second)
	}
	if cfg.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0 (no timeout for SSE)", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 60*time.Second)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
	}
}

func TestLoadMCP_AllCustom(t *testing.T) {
	t.Setenv("MCP_PROXY_BASE_URL", "http://synth:9090")
	t.Setenv("MCP_PORT", "7070")
	t.Setenv("MCP_REQUEST_TIMEOUT", "45s")
	t.Setenv("MCP_SERVER_READ_TIMEOUT", "5s")
	t.Setenv("MCP_SERVER_WRITE_TIMEOUT", "120s")
	t.Setenv("MCP_SERVER_IDLE_TIMEOUT", "30s")
	t.Setenv("MCP_SHUTDOWN_TIMEOUT", "5s")

	cfg, err := LoadMCP()
	if err != nil {
		t.Fatalf("LoadMCP() error: %v", err)
	}

	if cfg.ProxyBaseURL != "http://synth:9090" {
		t.Errorf("ProxyBaseURL = %q", cfg.ProxyBaseURL)
	}
	if cfg.Port != "7070" {
		t.Errorf("Port = %q", cfg.Port)
	}
	if cfg.RequestTimeout != 45*time.Second {
		t.Errorf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 120*time.Second {
		t.Errorf("WriteTimeout = %v", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 30*time.Second {
		t.Errorf("IdleTimeout = %v", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("ShutdownTimeout = %v", cfg.ShutdownTimeout)
	}
}

func TestLoadMCP_MissingProxyBaseURL(t *testing.T) {
	// Ensure MCP_PROXY_BASE_URL is unset
	t.Setenv("MCP_PROXY_BASE_URL", "")

	_, err := LoadMCP()
	if err == nil {
		t.Fatal("expected error for missing MCP_PROXY_BASE_URL")
	}
	if !strings.Contains(err.Error(), "MCP_PROXY_BASE_URL is required") {
		t.Errorf("error = %q, want mention of MCP_PROXY_BASE_URL", err.Error())
	}
}

func TestLoadMCP_InvalidProxyBaseURL(t *testing.T) {
	t.Setenv("MCP_PROXY_BASE_URL", "://bad")

	_, err := LoadMCP()
	if err == nil {
		t.Fatal("expected error for invalid MCP_PROXY_BASE_URL")
	}
}

func TestLoadMCP_InvalidDuration(t *testing.T) {
	t.Setenv("MCP_PROXY_BASE_URL", "http://localhost:8080")
	t.Setenv("MCP_REQUEST_TIMEOUT", "abc")

	_, err := LoadMCP()
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestMCPConfig_Addr(t *testing.T) {
	tests := []struct {
		port string
		want string
	}{
		{"8090", ":8090"},
		{":8090", ":8090"},
		{"3000", ":3000"},
	}
	for _, tc := range tests {
		t.Run(tc.port, func(t *testing.T) {
			cfg := MCPConfig{Port: tc.port}
			got := cfg.Addr()
			if got != tc.want {
				t.Errorf("Addr() = %q, want %q", got, tc.want)
			}
		})
	}
}
