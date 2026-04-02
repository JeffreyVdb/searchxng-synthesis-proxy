// Package proxy orchestrates search and LLM synthesis into a unified response.
package proxy

import (
	"context"
	"fmt"

	"github.com/example/search-synthesis-proxy/internal/searx"
)

// Searcher is the interface for searching.
type Searcher interface {
	Search(ctx context.Context, req searx.SearchRequest) (searx.SearchResponse, error)
}

// Generator is the interface for LLM text generation.
type Generator interface {
	GenerateJSON(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	Model() string
}

// Options configures the proxy service.
type Options struct {
	MaxSearchResults int
}

// Source is a cited source in the API response.
type Source struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Engine  string `json:"engine,omitempty"`
}

// Response is the API response payload.
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

// AppError is a typed application error with HTTP mapping.
type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func badRequest(code, msg string, err error) *AppError {
	return &AppError{Status: 400, Code: code, Message: msg, Err: err}
}

func badGateway(code, msg string, err error) *AppError {
	return &AppError{Status: 502, Code: code, Message: msg, Err: err}
}

// StatusOf extracts HTTP status, code, and message from an error.
// Returns 500/internal_error for non-AppError errors.
func StatusOf(err error) (status int, code string, message string) {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Status, appErr.Code, appErr.Message
	}
	return 500, "internal_error", "internal server error"
}
