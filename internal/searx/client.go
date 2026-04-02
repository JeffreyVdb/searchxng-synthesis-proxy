package searx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client queries a SearXNG instance.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new SearXNG client.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// Search queries SearXNG and returns normalized results.
func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	u, err := url.Parse(c.baseURL + "/search")
	if err != nil {
		return SearchResponse{}, fmt.Errorf("parse search url: %w", err)
	}
	q := u.Query()
	q.Set("q", req.Query)
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return SearchResponse{}, fmt.Errorf("searxng returned %d: %s", resp.StatusCode, string(body))
	}

	var wire searxWireResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return SearchResponse{}, fmt.Errorf("decode searxng response: %w", err)
	}

	var results []Result
	for _, r := range wire.Results {
		r.URL = strings.TrimSpace(r.URL)
		if r.URL == "" {
			continue
		}
		results = append(results, Result{
			Title:   normalizeWhitespace(r.Title),
			URL:     r.URL,
			Snippet: normalizeWhitespace(r.Content),
			Engine:  strings.TrimSpace(r.Engine),
		})
	}

	return SearchResponse{
		Query:   wire.Query,
		Results: results,
	}, nil
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
