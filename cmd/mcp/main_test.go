package main

import (
	"os"
	"strings"
	"testing"
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
