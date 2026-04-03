package synthproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Search_Success(t *testing.T) {
	want := &Response{
		Query:  "golang context",
		Answer: "Go context is used for cancellation.",
		Sources: []Source{
			{Index: 1, Title: "Go Blog", URL: "https://go.dev/blog/context", Snippet: "context package...", Engine: "google"},
		},
		Meta: Meta{Model: "test-model", ResultsConsidered: 1, TookMS: 100},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search" {
			t.Errorf("path = %q, want /v1/search", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "golang context" {
			t.Errorf("q = %q, want %q", r.URL.Query().Get("q"), "golang context")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, DefaultHTTPClient(10*time.Second))
	got, err := client.Search(context.Background(), "golang context")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if got.Query != want.Query {
		t.Errorf("Query = %q, want %q", got.Query, want.Query)
	}
	if got.Answer != want.Answer {
		t.Errorf("Answer = %q, want %q", got.Answer, want.Answer)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("len(Sources) = %d, want 1", len(got.Sources))
	}
	if got.Sources[0].Title != "Go Blog" {
		t.Errorf("Sources[0].Title = %q", got.Sources[0].Title)
	}
}

func TestClient_Search_UpstreamJSONError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "invalid_query",
				"message": "query parameter q is required",
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, DefaultHTTPClient(10*time.Second))
	_, err := client.Search(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	upErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *synthproxy.Error", err)
	}
	if upErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", upErr.StatusCode)
	}
	if upErr.Code != "invalid_query" {
		t.Errorf("Code = %q, want %q", upErr.Code, "invalid_query")
	}
	if upErr.Message != "query parameter q is required" {
		t.Errorf("Message = %q", upErr.Message)
	}
}

func TestClient_Search_UpstreamNonJSONError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, DefaultHTTPClient(10*time.Second))
	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	upErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *synthproxy.Error", err)
	}
	if upErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", upErr.StatusCode)
	}
	if upErr.Code != "upstream_error" {
		t.Errorf("Code = %q, want %q", upErr.Code, "upstream_error")
	}
}

func TestClient_Search_Upstream502(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "search_upstream_error",
				"message": "search backend request failed",
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, DefaultHTTPClient(10*time.Second))
	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error")
	}
	upErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *synthproxy.Error", err)
	}
	if upErr.StatusCode != 502 {
		t.Errorf("StatusCode = %d, want 502", upErr.StatusCode)
	}
}

func TestClient_Search_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, &http.Client{Timeout: 50 * time.Millisecond})
	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var transportErr *TransportError
	if err, ok := err.(*TransportError); !ok {
		t.Fatalf("error type = %T, want *TransportError", err)
	} else {
		transportErr = err
	}
	if transportErr.UserMessage() != "search upstream unavailable" {
		t.Errorf("UserMessage() = %q, want %q", transportErr.UserMessage(), "search upstream unavailable")
	}
}

func TestClient_Search_MalformedSuccessPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"query":"test","answer":123}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, DefaultHTTPClient(10*time.Second))
	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}

	decodeErr, ok := err.(*DecodeError)
	if !ok {
		t.Fatalf("error type = %T, want *DecodeError", err)
	}
	if decodeErr.UserMessage() != "search upstream returned an invalid response" {
		t.Errorf("UserMessage() = %q, want %q", decodeErr.UserMessage(), "search upstream returned an invalid response")
	}
}

func TestClient_Search_ConnectionRefused(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", DefaultHTTPClient(1*time.Second))
	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected connection error")
	}

	transportErr, ok := err.(*TransportError)
	if !ok {
		t.Fatalf("error type = %T, want *TransportError", err)
	}
	if transportErr.UserMessage() != "search upstream unavailable" {
		t.Errorf("UserMessage() = %q, want %q", transportErr.UserMessage(), "search upstream unavailable")
	}
}
