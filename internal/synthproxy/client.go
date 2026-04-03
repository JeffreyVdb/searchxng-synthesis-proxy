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
// The Message field contains the upstream error text.
// Use UserMessage() to get a sanitized message safe for external callers.
// The underlying details are kept for logging only.
type Error struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("upstream %d: %s: %s", e.StatusCode, e.Code, e.Message)
}

// UserMessage returns a sanitized message safe for external callers.
// It strips internal transport details and hostnames.
func (e *Error) UserMessage() string {
	switch {
	case e.StatusCode == 0:
		// Non-HTTP transport error (connection refused, timeout, DNS, etc.)
		return "search upstream unavailable"
	default:
		return fmt.Sprintf("search upstream returned an error (%s)", e.Code)
	}
}

// TransportError represents a failure to reach the upstream proxy
// (connection refused, DNS failure, timeout, TLS error, etc.).
// It carries a safe public message and the original cause for logging.
type TransportError struct {
	// Cause is the original error for logging.
	Cause error
}

func (e *TransportError) Error() string {
	return fmt.Sprintf("transport error: %v", e.Cause)
}

func (e *TransportError) Unwrap() error {
	return e.Cause
}

// UserMessage returns a sanitized message safe for external callers.
func (e *TransportError) UserMessage() string {
	return "search upstream unavailable"
}

// DecodeError represents a failure to decode the upstream response body.
// It carries a safe public message and the original cause for logging.
type DecodeError struct {
	Cause error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("decode error: %v", e.Cause)
}

func (e *DecodeError) Unwrap() error {
	return e.Cause
}

// UserMessage returns a sanitized message safe for external callers.
func (e *DecodeError) UserMessage() string {
	return "search upstream returned an invalid response"
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
		return nil, &TransportError{Cause: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, &DecodeError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseUpstreamError(resp.StatusCode, body)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &DecodeError{Cause: err}
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
