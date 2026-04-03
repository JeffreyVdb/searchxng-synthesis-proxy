// Package config reads and validates all configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Port              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	SearchTimeout     time.Duration
	LLMTimeout        time.Duration
	ShutdownTimeout   time.Duration
	MaxSearchResults  int

	SearXNGBaseURL    string

	LLMAPIKey         string
	LLMBaseURL        string
	LLMModel          string
	OpenRouterReferer string
	OpenRouterTitle   string
}

// Addr returns the listen address in the form ":port".
func (c Config) Addr() string {
	if strings.HasPrefix(c.Port, ":") {
		return c.Port
	}
	return ":" + c.Port
}

// Load reads environment variables and returns a validated Config.
//
// Deprecated: Use LoadProxy() for clarity now that a second binary exists.
// Load is kept for backward compatibility with cmd/proxy.
func Load() (Config, error) {
	return LoadProxy()
}

// LoadProxy reads environment variables and returns a validated Config for the
// main search synthesis proxy service.
func LoadProxy() (Config, error) {
	var cfg Config
	var errs []string

	cfg.LLMAPIKey = strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	if cfg.LLMAPIKey == "" {
		errs = append(errs, "LLM_API_KEY is required")
	}

	cfg.Port = getenvDefault("PROXY_PORT", "8080")

	cfg.SearXNGBaseURL = getenvDefault("SEARXNG_BASE_URL", "http://127.0.0.1:8888")
	if err := mustAbsoluteURL("SEARXNG_BASE_URL", cfg.SearXNGBaseURL); err != nil {
		errs = append(errs, err.Error())
	}

	cfg.LLMBaseURL = getenvDefault("LLM_BASE_URL", "https://openrouter.ai/api/v1")
	if err := mustAbsoluteURL("LLM_BASE_URL", cfg.LLMBaseURL); err != nil {
		errs = append(errs, err.Error())
	}

	cfg.LLMModel = getenvDefault("LLM_MODEL", "xiaomi/mimo-v2-flash")

	cfg.OpenRouterReferer = strings.TrimSpace(os.Getenv("OPENROUTER_REFERER"))
	cfg.OpenRouterTitle = strings.TrimSpace(os.Getenv("OPENROUTER_TITLE"))

	var err error
	cfg.MaxSearchResults, err = parsePositiveInt("MAX_SEARCH_RESULTS", getenvDefault("MAX_SEARCH_RESULTS", "5"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.ReadTimeout, err = parsePositiveDuration("SERVER_READ_TIMEOUT", getenvDefault("SERVER_READ_TIMEOUT", "10s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.WriteTimeout, err = parsePositiveDuration("SERVER_WRITE_TIMEOUT", getenvDefault("SERVER_WRITE_TIMEOUT", "60s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.IdleTimeout, err = parsePositiveDuration("SERVER_IDLE_TIMEOUT", getenvDefault("SERVER_IDLE_TIMEOUT", "60s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.SearchTimeout, err = parsePositiveDuration("SEARCH_TIMEOUT", getenvDefault("SEARCH_TIMEOUT", "10s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.LLMTimeout, err = parsePositiveDuration("LLM_TIMEOUT", getenvDefault("LLM_TIMEOUT", "45s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cfg.ShutdownTimeout, err = parsePositiveDuration("SHUTDOWN_TIMEOUT", getenvDefault("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("config validation failed: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func parsePositiveDuration(name, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", name, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: duration must be positive, got %s", name, d)
	}
	return d, nil
}

func parsePositiveInt(name, raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid integer %q: %w", name, raw, err)
	}
	if n < 1 {
		return 0, fmt.Errorf("%s: must be >= 1, got %d", name, n)
	}
	return n, nil
}

func mustAbsoluteURL(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: invalid URL %q: %w", name, raw, err)
	}
	if !u.IsAbs() {
		return fmt.Errorf("%s: must be an absolute URL, got %q", name, raw)
	}
	return nil
}
