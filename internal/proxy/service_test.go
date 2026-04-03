package proxy

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/searx"
)// fakeSearcher implements Searcher for tests.
type fakeSearcher struct {
	resp searx.SearchResponse
	err  error
}

func (f *fakeSearcher) Search(_ context.Context, _ searx.SearchRequest) (searx.SearchResponse, error) {
	return f.resp, f.err
}

// fakeGenerator implements Generator for tests.
type fakeGenerator struct {
	content string
	err     error
	model   string
}

func (f *fakeGenerator) GenerateJSON(_ context.Context, _, _ string) (string, error) {
	return f.content, f.err
}

func (f *fakeGenerator) Model() string {
	if f.model == "" {
		return "test-model"
	}
	return f.model
}

func TestService_HappyPath(t *testing.T) {
	searcher := &fakeSearcher{
		resp: searx.SearchResponse{
			Query: "golang context",
			Results: []searx.Result{
				{Title: "Go Context", URL: "https://go.dev/blog/context", Snippet: "The context package...", Engine: "google"},
				{Title: "Context Tutorial", URL: "https://example.com/tutorial", Snippet: "Learn context", Engine: "duckduckgo"},
			},
		},
	}
	generator := &fakeGenerator{
		content: `{"answer":"context is used for cancellation and deadlines","citations":[1,2]}`,
	}

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	resp, err := svc.SearchAndSynthesize(context.Background(), "golang context")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Answer != "context is used for cancellation and deadlines" {
		t.Errorf("Answer = %q", resp.Answer)
	}
	if len(resp.Sources) != 2 {
		t.Fatalf("len(Sources) = %d, want 2", len(resp.Sources))
	}
	if resp.Sources[0].Index != 1 {
		t.Errorf("Sources[0].Index = %d, want 1", resp.Sources[0].Index)
	}
	if resp.Sources[0].URL != "https://go.dev/blog/context" {
		t.Errorf("Sources[0].URL = %q", resp.Sources[0].URL)
	}
	if resp.Meta.ResultsConsidered != 2 {
		t.Errorf("Meta.ResultsConsidered = %d, want 2", resp.Meta.ResultsConsidered)
	}
	if resp.Meta.Model != "test-model" {
		t.Errorf("Meta.Model = %q", resp.Meta.Model)
	}
}

func TestService_EmptyQuery(t *testing.T) {
	svc := NewService(&fakeSearcher{}, &fakeGenerator{}, Options{MaxSearchResults: 5})
	_, err := svc.SearchAndSynthesize(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	status, code, _ := StatusOf(err)
	if status != 400 {
		t.Errorf("status = %d, want 400", status)
	}
	if code != "invalid_query" {
		t.Errorf("code = %q, want %q", code, "invalid_query")
	}
}

func TestService_QueryTooLong(t *testing.T) {
	svc := NewService(&fakeSearcher{}, &fakeGenerator{}, Options{MaxSearchResults: 5})
	longQuery := strings.Repeat("a", 2049)
	_, err := svc.SearchAndSynthesize(context.Background(), longQuery)
	if err == nil {
		t.Fatal("expected error for long query")
	}
	status, code, _ := StatusOf(err)
	if status != 400 {
		t.Errorf("status = %d, want 400", status)
	}
	if code != "query_too_long" {
		t.Errorf("code = %q, want %q", code, "query_too_long")
	}
}

func TestService_SearcherFailure(t *testing.T) {
	searcher := &fakeSearcher{err: fmt.Errorf("connection refused")}
	svc := NewService(searcher, &fakeGenerator{}, Options{MaxSearchResults: 5})
	_, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error")
	}
	status, code, _ := StatusOf(err)
	if status != 502 {
		t.Errorf("status = %d, want 502", status)
	}
	if code != "search_upstream_error" {
		t.Errorf("code = %q, want %q", code, "search_upstream_error")
	}
}

func TestService_NoSearchResults(t *testing.T) {
	searcher := &fakeSearcher{resp: searx.SearchResponse{Query: "test"}}
	generator := &fakeGenerator{} // should not be called

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	resp, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != "No relevant search results were found for the given query." {
		t.Errorf("Answer = %q", resp.Answer)
	}
	if len(resp.Sources) != 0 {
		t.Errorf("len(Sources) = %d, want 0", len(resp.Sources))
	}
	if resp.Meta.ResultsConsidered != 0 {
		t.Errorf("ResultsConsidered = %d, want 0", resp.Meta.ResultsConsidered)
	}
}

func TestService_GeneratorFailure(t *testing.T) {
	searcher := &fakeSearcher{
		resp: searx.SearchResponse{
			Query:   "test",
			Results: []searx.Result{{Title: "A", URL: "https://a.com", Snippet: "a", Engine: "google"}},
		},
	}
	generator := &fakeGenerator{err: fmt.Errorf("provider error")}

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	_, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error")
	}
	status, code, _ := StatusOf(err)
	if status != 502 {
		t.Errorf("status = %d, want 502", status)
	}
	if code != "llm_upstream_error" {
		t.Errorf("code = %q", code)
	}
}

func TestService_EmbeddedJSONObject(t *testing.T) {
	searcher := &fakeSearcher{
		resp: searx.SearchResponse{
			Query:   "test",
			Results: []searx.Result{{Title: "A", URL: "https://a.com", Snippet: "a", Engine: "google"}},
		},
	}
	generator := &fakeGenerator{
		content: "Here is the result:\n```json\n{\"answer\":\"ok\",\"citations\":[1]}\n```\n",
	}

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	resp, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != "ok" {
		t.Errorf("Answer = %q, want %q", resp.Answer, "ok")
	}
}

func TestService_FencedJSON(t *testing.T) {
	searcher := &fakeSearcher{
		resp: searx.SearchResponse{
			Query:   "test",
			Results: []searx.Result{{Title: "A", URL: "https://a.com", Snippet: "a", Engine: "google"}},
		},
	}
	generator := &fakeGenerator{
		content: "```json\n{\"answer\":\"ok\",\"citations\":[1]}\n```",
	}

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	resp, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != "ok" {
		t.Errorf("Answer = %q, want %q", resp.Answer, "ok")
	}
}

func TestService_InvalidCitations(t *testing.T) {
	searcher := &fakeSearcher{
		resp: searx.SearchResponse{
			Query: "test",
			Results: []searx.Result{
				{Title: "A", URL: "https://a.com", Snippet: "a", Engine: "google"},
				{Title: "B", URL: "https://b.com", Snippet: "b", Engine: "google"},
			},
		},
	}
	generator := &fakeGenerator{
		content: `{"answer":"see sources","citations":[99,-1,2,2]}`,
	}

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	resp, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Sources) != 1 {
		t.Fatalf("len(Sources) = %d, want 1 (only valid unique citations)", len(resp.Sources))
	}
	if resp.Sources[0].Index != 2 {
		t.Errorf("Sources[0].Index = %d, want 2", resp.Sources[0].Index)
	}
}

func TestService_InvalidJSON(t *testing.T) {
	searcher := &fakeSearcher{
		resp: searx.SearchResponse{
			Query:   "test",
			Results: []searx.Result{{Title: "A", URL: "https://a.com", Snippet: "a", Engine: "google"}},
		},
	}
	generator := &fakeGenerator{content: "not json at all"}

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	_, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	_, code, _ := StatusOf(err)
	if code != "llm_invalid_json" {
		t.Errorf("code = %q, want %q", code, "llm_invalid_json")
	}
}

func TestService_MissingAnswer(t *testing.T) {
	searcher := &fakeSearcher{
		resp: searx.SearchResponse{
			Query:   "test",
			Results: []searx.Result{{Title: "A", URL: "https://a.com", Snippet: "a", Engine: "google"}},
		},
	}
	generator := &fakeGenerator{content: `{"citations":[1]}`}

	svc := NewService(searcher, generator, Options{MaxSearchResults: 5})
	_, err := svc.SearchAndSynthesize(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for missing answer")
	}
	_, code, _ := StatusOf(err)
	if code != "llm_invalid_payload" {
		t.Errorf("code = %q, want %q", code, "llm_invalid_payload")
	}
}

func TestSanitizeCitations(t *testing.T) {
	tests := []struct {
		name      string
		citations []int
		maxIndex  int
		want      []int
	}{
		{"valid", []int{1, 2, 3}, 3, []int{1, 2, 3}},
		{"with out of range", []int{1, 99, 2}, 3, []int{1, 2}},
		{"with negative", []int{-1, 0, 1}, 3, []int{1}},
		{"duplicates", []int{2, 2, 1, 1}, 3, []int{2, 1}},
		{"all invalid", []int{99, -1, 0}, 3, nil},
		{"empty", []int{}, 3, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeCitations(tc.citations, tc.maxIndex)
			if len(got) != len(tc.want) {
				t.Fatalf("sanitizeCitations() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("sanitizeCitations()[%d] = %d, want %d", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestTruncateTitle_Unicode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"ascii", "hello world", 5, "hello…"},
		{"emoji", "🚀🚀🚀🚀🚀", 3, "🚀🚀🚀…"},
		{"mixed", "hello 🚀 world", 7, "hello 🚀…"},
		{"within limit", "🚀", 5, "🚀"},
		{"exact limit", "abcde", 5, "abcde"},
		{"empty", "", 5, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateTitle(tc.input, tc.max)
			if got != tc.want {
				t.Errorf("truncateTitle(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
			}
			// Verify result is valid UTF-8
			if !utf8.ValidString(got) {
				t.Errorf("truncateTitle(%q, %d) produced invalid UTF-8: %q", tc.input, tc.max, got)
			}
		})
	}
}

func TestTruncateSnippet_Unicode(t *testing.T) {
	got := truncateSnippet("café résumé", 4)
	want := "café…"
	if got != want {
		t.Errorf("truncateSnippet() = %q, want %q", got, want)
	}
	if !utf8.ValidString(got) {
		t.Error("truncateSnippet() produced invalid UTF-8")
	}
}

func TestStatusOf_NonAppError(t *testing.T) {
	status, code, msg := StatusOf(fmt.Errorf("generic error"))
	if status != 500 {
		t.Errorf("status = %d, want 500", status)
	}
	if code != "internal_error" {
		t.Errorf("code = %q, want %q", code, "internal_error")
	}
	if msg != "internal server error" {
		t.Errorf("msg = %q", msg)
	}
}
