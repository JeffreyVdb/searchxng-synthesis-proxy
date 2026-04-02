package searx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSearch_CorrectRequestShape(t *testing.T) {
	var gotReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		resp := searxWireResponse{Query: "test"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	_, err := client.Search(context.Background(), SearchRequest{Query: "golang"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if gotReq == nil {
		t.Fatal("no request received")
	}
	if gotReq.URL.Path != "/search" {
		t.Errorf("path = %q, want /search", gotReq.URL.Path)
	}
	if gotReq.URL.Query().Get("q") != "golang" {
		t.Errorf("q param = %q, want %q", gotReq.URL.Query().Get("q"), "golang")
	}
	if gotReq.URL.Query().Get("format") != "json" {
		t.Errorf("format param = %q, want %q", gotReq.URL.Query().Get("format"), "json")
	}
}

func TestSearch_SuccessfulParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := searxWireResponse{
			Query: "golang context",
			Results: []struct {
				Title   string `json:"title"`
				URL     string `json:"url"`
				Content string `json:"content"`
				Engine  string `json:"engine"`
			}{
				{Title: "Go Context", URL: "https://go.dev/blog/context", Content: "The context package...", Engine: "google"},
				{Title: "Context Tutorial", URL: "https://example.com/tutorial", Content: "Learn about context", Engine: "duckduckgo"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	resp, err := client.Search(context.Background(), SearchRequest{Query: "golang context"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(resp.Results))
	}
	if resp.Results[0].Title != "Go Context" {
		t.Errorf("Results[0].Title = %q", resp.Results[0].Title)
	}
	if resp.Results[0].URL != "https://go.dev/blog/context" {
		t.Errorf("Results[0].URL = %q", resp.Results[0].URL)
	}
}

func TestSearch_DropsEmptyURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := searxWireResponse{
			Query: "test",
			Results: []struct {
				Title   string `json:"title"`
				URL     string `json:"url"`
				Content string `json:"content"`
				Engine  string `json:"engine"`
			}{
				{Title: "Valid", URL: "https://example.com", Content: "ok", Engine: "google"},
				{Title: "No URL", URL: "", Content: "bad", Engine: "google"},
				{Title: "Whitespace URL", URL: "  ", Content: "also bad", Engine: "google"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	resp, err := client.Search(context.Background(), SearchRequest{Query: "test"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1 (empty URLs dropped)", len(resp.Results))
	}
	if resp.Results[0].Title != "Valid" {
		t.Errorf("Results[0].Title = %q, want 'Valid'", resp.Results[0].Title)
	}
}

func TestSearch_Non200Upstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	_, err := client.Search(context.Background(), SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestSearch_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	_, err := client.Search(context.Background(), SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSearch_ContextCancellation(t *testing.T) {
	var mu sync.Mutex
	proceeded := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		mu.Lock()
		proceeded = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	mu.Lock()
	if proceeded {
		mu.Unlock()
		t.Log("server handler completed despite cancellation (timing-dependent)")
	}
	mu.Unlock()
}
