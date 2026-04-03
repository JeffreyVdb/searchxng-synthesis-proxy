// Package synthproxy provides an HTTP client for consuming the search synthesis
// proxy's /v1/search API from a remote MCP wrapper process.
package synthproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Source is a cited source in the upstream proxy response.
type Source struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Engine  string `json:"engine,omitempty"`
}

// Response mirrors the upstream proxy JSON payload.
type Response struct {
	Query   string   `json:"query"`
	Answer  string   `json:"answer"`
	Sources []Source `json:"sources"`
	Meta    Meta     `json:"meta"`
}

// Meta contains metadata about the synthesis.
type Meta struct {
	Model             string `json:"model"`
	ResultsConsidered int    `json:"results_considered"`
	TookMS            int64  `json:"took_ms"`
}

// Error represents an upstream error returned by the proxy.
type Error struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("upstream %d: %s: %s", e.StatusCode, e.Code, e.Message)
}

// Client calls the search synthesis proxy over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new upstream client. baseURL is the proxy root, e.g.
// "http://127.0.0.1:8080".
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// Search calls GET {baseURL}/v1/search?q=... and returns the decoded response.
func (c *Client) Search(ctx context.Context, query string) (*Response, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream base URL: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, "/v1/search")
	if err != nil {
		return nil, fmt.Errorf("building search URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to upstream: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, fmt.Errorf("reading upstream response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseUpstreamError(resp.StatusCode, body)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding upstream response: %w", err)
	}

	return &result, nil
}

// upstreamErrorPayload matches the proxy error JSON shape.
type upstreamErrorPayload struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseUpstreamError(status int, body []byte) *Error {
	var payload upstreamErrorPayload
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Code != "" {
		return &Error{
			StatusCode: status,
			Code:       payload.Error.Code,
			Message:    payload.Error.Message,
		}
	}
	return &Error{
		StatusCode: status,
		Code:       "upstream_error",
		Message:    fmt.Sprintf("upstream returned HTTP %d", status),
	}
}

// DefaultHTTPClient returns an http.Client suitable for calling the upstream
// proxy with the given timeout.
func DefaultHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
