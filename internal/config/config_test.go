package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.LLMAPIKey != "test-key" {
		t.Errorf("LLMAPIKey = %q, want %q", cfg.LLMAPIKey, "test-key")
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.SearXNGBaseURL != "http://127.0.0.1:8888" {
		t.Errorf("SearXNGBaseURL = %q, want %q", cfg.SearXNGBaseURL, "http://127.0.0.1:8888")
	}
	if cfg.LLMBaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("LLMBaseURL = %q, want %q", cfg.LLMBaseURL, "https://openrouter.ai/api/v1")
	}
	if cfg.LLMModel != "xiaomi/mimo-v2-flash" {
		t.Errorf("LLMModel = %q, want %q", cfg.LLMModel, "xiaomi/mimo-v2-flash")
	}
	if cfg.MaxSearchResults != 5 {
		t.Errorf("MaxSearchResults = %d, want %d", cfg.MaxSearchResults, 5)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, 10*time.Second)
	}
	if cfg.WriteTimeout != 60*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, 60*time.Second)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 60*time.Second)
	}
	if cfg.SearchTimeout != 10*time.Second {
		t.Errorf("SearchTimeout = %v, want %v", cfg.SearchTimeout, 10*time.Second)
	}
	if cfg.LLMTimeout != 45*time.Second {
		t.Errorf("LLMTimeout = %v, want %v", cfg.LLMTimeout, 45*time.Second)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
	}
}

func TestLoad_AllCustom(t *testing.T) {
	t.Setenv("LLM_API_KEY", "my-key")
	t.Setenv("PROXY_PORT", "9090")
	t.Setenv("SEARXNG_BASE_URL", "http://searx:8080")
	t.Setenv("LLM_BASE_URL", "https://api.openai.com/v1")
	t.Setenv("LLM_MODEL", "gpt-4o")
	t.Setenv("MAX_SEARCH_RESULTS", "10")
	t.Setenv("SERVER_READ_TIMEOUT", "5s")
	t.Setenv("SERVER_WRITE_TIMEOUT", "30s")
	t.Setenv("SERVER_IDLE_TIMEOUT", "15s")
	t.Setenv("SEARCH_TIMEOUT", "8s")
	t.Setenv("LLM_TIMEOUT", "20s")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("OPENROUTER_REFERER", "https://example.com")
	t.Setenv("OPENROUTER_TITLE", "MyApp")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.SearXNGBaseURL != "http://searx:8080" {
		t.Errorf("SearXNGBaseURL = %q", cfg.SearXNGBaseURL)
	}
	if cfg.LLMBaseURL != "https://api.openai.com/v1" {
		t.Errorf("LLMBaseURL = %q", cfg.LLMBaseURL)
	}
	if cfg.LLMModel != "gpt-4o" {
		t.Errorf("LLMModel = %q", cfg.LLMModel)
	}
	if cfg.MaxSearchResults != 10 {
		t.Errorf("MaxSearchResults = %d, want %d", cfg.MaxSearchResults, 10)
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v", cfg.ReadTimeout)
	}
	if cfg.OpenRouterReferer != "https://example.com" {
		t.Errorf("OpenRouterReferer = %q", cfg.OpenRouterReferer)
	}
	if cfg.OpenRouterTitle != "MyApp" {
		t.Errorf("OpenRouterTitle = %q", cfg.OpenRouterTitle)
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing LLM_API_KEY")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("SEARCH_TIMEOUT", "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestLoad_InvalidURL(t *testing.T) {
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("LLM_BASE_URL", "://bad")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestLoad_InvalidMaxResults(t *testing.T) {
	tests := []struct {
		name string
		val  string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"not a number", "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LLM_API_KEY", "key")
			t.Setenv("MAX_SEARCH_RESULTS", tc.val)
			_, err := Load()
			if err == nil {
				t.Fatal("expected error for invalid MAX_SEARCH_RESULTS")
			}
		})
	}
}

func TestAddr(t *testing.T) {
	tests := []struct {
		port string
		want string
	}{
		{"8080", ":8080"},
		{":8080", ":8080"},
		{"3000", ":3000"},
	}
	for _, tc := range tests {
		t.Run(tc.port, func(t *testing.T) {
			cfg := Config{Port: tc.port}
			got := cfg.Addr()
			if got != tc.want {
				t.Errorf("Addr() = %q, want %q", got, tc.want)
			}
		})
	}
}
