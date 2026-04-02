package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/config"
)

func testConfig() config.Config {
	return config.Config{
		Port:             "0",
		ReadTimeout:      10 * time.Second,
		WriteTimeout:     60 * time.Second,
		IdleTimeout:      60 * time.Second,
		SearchTimeout:    10 * time.Second,
		LLMTimeout:       45 * time.Second,
		LLMAPIKey:        "test-key",
		LLMBaseURL:       "https://openrouter.ai/api/v1",
		LLMModel:         "test-model",
		MaxSearchResults: 5,
	}
}

func TestNewServer_ValidConfig(t *testing.T) {
	logger := slog.Default()
	srv, err := newServer(testConfig(), logger)
	if err != nil {
		t.Fatalf("newServer() error: %v", err)
	}
	if srv == nil {
		t.Fatal("server is nil")
	}
	if srv.Handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestHealthzEndpoint(t *testing.T) {
	logger := slog.Default()
	srv, err := newServer(testConfig(), logger)
	if err != nil {
		t.Fatalf("newServer() error: %v", err)
	}

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
