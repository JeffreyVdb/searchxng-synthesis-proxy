package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// MCPConfig holds configuration specific to the MCP SSE server binary.
// It does not require main-service-only secrets such as LLM_API_KEY.
type MCPConfig struct {
	Port             string
	ProxyBaseURL     string
	RequestTimeout   time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ShutdownTimeout  time.Duration
}

// Addr returns the listen address in the form ":port".
func (c MCPConfig) Addr() string {
	if strings.HasPrefix(c.Port, ":") {
		return c.Port
	}
	return ":" + c.Port
}

// LoadMCP reads MCP-specific environment variables and returns a validated MCPConfig.
func LoadMCP() (MCPConfig, error) {
	var cfg MCPConfig
	var errs []string

	cfg.ProxyBaseURL = strings.TrimSpace(os.Getenv("MCP_PROXY_BASE_URL"))
	if cfg.ProxyBaseURL == "" {
		errs = append(errs, "MCP_PROXY_BASE_URL is required")
	} else if err := mustAbsoluteURL("MCP_PROXY_BASE_URL", cfg.ProxyBaseURL); err != nil {
		errs = append(errs, err.Error())
	}

	cfg.Port = getenvDefault("MCP_PORT", "8090")

	var err error
	cfg.RequestTimeout, err = parsePositiveDuration("MCP_REQUEST_TIMEOUT", getenvDefault("MCP_REQUEST_TIMEOUT", "30s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.ReadTimeout, err = parsePositiveDuration("MCP_SERVER_READ_TIMEOUT", getenvDefault("MCP_SERVER_READ_TIMEOUT", "10s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	// WriteTimeout defaults to 0 (no timeout) to keep SSE connections alive.
	rawWT := getenvDefault("MCP_SERVER_WRITE_TIMEOUT", "0s")
	if rawWT == "0s" || rawWT == "0" {
		cfg.WriteTimeout = 0
	} else {
		cfg.WriteTimeout, err = parsePositiveDuration("MCP_SERVER_WRITE_TIMEOUT", rawWT)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	cfg.IdleTimeout, err = parsePositiveDuration("MCP_SERVER_IDLE_TIMEOUT", getenvDefault("MCP_SERVER_IDLE_TIMEOUT", "60s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.ShutdownTimeout, err = parsePositiveDuration("MCP_SHUTDOWN_TIMEOUT", getenvDefault("MCP_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return MCPConfig{}, fmt.Errorf("MCP config validation failed: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}
