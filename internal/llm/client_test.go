package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_CorrectRequestPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeFakeCompletion(w, "ok")
	}))
	defer srv.Close()

	opts := Options{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	}
	client := NewClient(opts, srv.Client())
	_, err := client.GenerateJSON(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("GenerateJSON() error: %v", err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("request path = %q, want /v1/chat/completions", gotPath)
	}
}

func TestClient_AuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeFakeCompletion(w, "ok")
	}))
	defer srv.Close()

	opts := Options{
		APIKey:  "my-secret-key",
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	}
	client := NewClient(opts, srv.Client())
	_, err := client.GenerateJSON(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("GenerateJSON() error: %v", err)
	}
	if gotAuth != "Bearer my-secret-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer my-secret-key")
	}
}

func TestClient_OpenRouterAttributionHeaders(t *testing.T) {
	var gotReferer, gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		writeFakeCompletion(w, "ok")
	}))
	defer srv.Close()

	opts := Options{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
		Referer: "https://example.com",
		Title:   "MyApp",
	}
	client := NewClient(opts, srv.Client())
	_, err := client.GenerateJSON(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("GenerateJSON() error: %v", err)
	}
	if gotReferer != "https://example.com" {
		t.Errorf("HTTP-Referer = %q, want %q", gotReferer, "https://example.com")
	}
	if gotTitle != "MyApp" {
		t.Errorf("X-Title = %q, want %q", gotTitle, "MyApp")
	}
}

func TestClient_SuccessfulCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeFakeCompletion(w, `{"answer":"hello","citations":[1]}`)
	}))
	defer srv.Close()

	opts := Options{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	}
	client := NewClient(opts, srv.Client())
	content, err := client.GenerateJSON(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("GenerateJSON() error: %v", err)
	}
	if content != `{"answer":"hello","citations":[1]}` {
		t.Errorf("content = %q, want JSON", content)
	}
}

func TestClient_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id":      "chatcmpl_test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "test-model",
			"choices": []interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	opts := Options{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	}
	client := NewClient(opts, srv.Client())
	_, err := client.GenerateJSON(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error = %q, want 'no choices'", err.Error())
	}
}

func TestClient_ProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"internal error"}}`)
	}))
	defer srv.Close()

	opts := Options{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	}
	client := NewClient(opts, srv.Client())
	_, err := client.GenerateJSON(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error for provider 500")
	}
}

func TestClient_TimeoutContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		writeFakeCompletion(w, "ok")
	}))
	defer srv.Close()

	opts := Options{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	}
	client := NewClient(opts, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GenerateJSON(ctx, "sys", "user")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestClient_Model(t *testing.T) {
	opts := Options{Model: "xiaomi/mimo-v2-flash"}
	client := NewClient(opts, nil)
	if got := client.Model(); got != "xiaomi/mimo-v2-flash" {
		t.Errorf("Model() = %q, want %q", got, "xiaomi/mimo-v2-flash")
	}
}

func writeFakeCompletion(w http.ResponseWriter, content string) {
	resp := map[string]interface{}{
		"id":      "chatcmpl_test",
		"object":  "chat.completion",
		"created": 1,
		"model":   "test-model",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
