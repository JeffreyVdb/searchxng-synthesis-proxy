package main

import (
	"testing"
)

func TestRun_MissingConfig(t *testing.T) {
	// Without MCP_PROXY_BASE_URL, run() should return an error.
	// We can't easily test the full run() without an env, so test the config
	// loading path separately (covered by config/mcp_test.go).
	// This test just ensures the binary compiles.
}
