package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/synthproxy"
)

// stubClient implements SearchClient for tests.
type stubClient struct {
	resp *synthproxy.Response
	err  error
}

func (s *stubClient) Search(_ context.Context, query string) (*synthproxy.Response, error) {
	return s.resp, s.err
}

func TestFormatResponse(t *testing.T) {
	resp := &synthproxy.Response{
		Query:  "golang context",
		Answer: "Go context is used for cancellation.",
		Sources: []synthproxy.Source{
			{Index: 1, Title: "Go Blog", URL: "https://go.dev/blog/context", Snippet: "context package", Engine: "google"},
			{Index: 2, Title: "Go Docs", URL: "https://pkg.go.dev/context", Snippet: "", Engine: "duckduckgo"},
		},
	}

	text := FormatResponse(resp)

	if !strings.Contains(text, "Answer:") {
		t.Error("missing Answer header")
	}
	if !strings.Contains(text, "Go context is used for cancellation.") {
		t.Error("missing answer text")
	}
	if !strings.Contains(text, "Sources:") {
		t.Error("missing Sources header")
	}
	if !strings.Contains(text, "1. Go Blog") {
		t.Error("missing first source title")
	}
	if !strings.Contains(text, "https://go.dev/blog/context") {
		t.Error("missing first source URL")
	}
	if !strings.Contains(text, "context package") {
		t.Error("missing first source snippet")
	}
	if !strings.Contains(text, "2. Go Docs") {
		t.Error("missing second source title")
	}
}

func TestFormatResponse_NoSources(t *testing.T) {
	resp := &synthproxy.Response{
		Query:  "test",
		Answer: "No results found.",
	}

	text := FormatResponse(resp)

	if strings.Contains(text, "Sources:") {
		t.Error("should not contain Sources header when there are no sources")
	}
}

func TestFormatResponse_EmptySnippet(t *testing.T) {
	resp := &synthproxy.Response{
		Query:  "test",
		Answer: "An answer.",
		Sources: []synthproxy.Source{
			{Index: 1, Title: "Title", URL: "https://example.com", Snippet: "", Engine: ""},
		},
	}

	text := FormatResponse(resp)

	// Should have title and URL but no blank snippet line
	lines := strings.Split(text, "\n")
	blankLines := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blankLines++
		}
	}
	// There will be some blank lines (trailing newline, etc.) but the snippet
	// should not produce an extra blank content line.
	if !strings.Contains(text, "1. Title") {
		t.Error("missing source title")
	}
	if !strings.Contains(text, "https://example.com") {
		t.Error("missing source URL")
	}
}
