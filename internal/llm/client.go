// Package llm provides a client for OpenAI-compatible LLM providers.
package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Options configures the LLM client.
type Options struct {
	APIKey  string
	BaseURL string
	Model   string
	Referer string
	Title   string
}

// Client wraps an OpenAI-compatible chat completion provider.
type Client struct {
	model   string
	sdk     *openai.Client
	reqOpts []option.RequestOption
}

// NewClient creates a new LLM client with the given HTTP client.
func NewClient(opts Options, httpClient *http.Client) *Client {
	sdkOpts := []option.RequestOption{
		option.WithAPIKey(opts.APIKey),
		option.WithBaseURL(opts.BaseURL),
		option.WithHTTPClient(httpClient),
		option.WithMaxRetries(0),
	}

	var reqOpts []option.RequestOption
	if opts.Referer != "" {
		reqOpts = append(reqOpts, option.WithHeader("HTTP-Referer", opts.Referer))
	}
	if opts.Title != "" {
		reqOpts = append(reqOpts, option.WithHeader("X-Title", opts.Title))
	}

	sdk := openai.NewClient(sdkOpts...)

	return &Client{
		model:   opts.Model,
		sdk:     &sdk,
		reqOpts: reqOpts,
	}
}

// GenerateJSON sends a chat completion request and returns the assistant message content.
func (c *Client) GenerateJSON(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	resp, err := c.sdk.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(c.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.DeveloperMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
	}, c.reqOpts...)
	if err != nil {
		return "", fmt.Errorf("chat completion request: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("chat completion returned no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("chat completion returned empty content")
	}

	return content, nil
}

// Model returns the configured model name.
func (c *Client) Model() string {
	return c.model
}
