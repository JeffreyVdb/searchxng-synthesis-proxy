package proxy

import (
	"context"
	"strings"
	"time"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/searx"
)

type sourceForResult = struct {
	index   int
	title   string
	url     string
	snippet string
	engine  string
}

// Service orchestrates search and LLM synthesis.
type Service struct {
	searcher  Searcher
	generator Generator
	opts      Options
}

// NewService creates a new proxy service.
func NewService(searcher Searcher, generator Generator, opts Options) *Service {
	return &Service{
		searcher:  searcher,
		generator: generator,
		opts:      opts,
	}
}

// SearchAndSynthesize performs the full search-synthesis pipeline.
func (s *Service) SearchAndSynthesize(ctx context.Context, query string) (Response, error) {
	start := time.Now()

	// Validate query
	query = strings.TrimSpace(query)
	if query == "" {
		return Response{}, badRequest("invalid_query", "query parameter q is required", nil)
	}
	if len(query) > 2048 {
		return Response{}, badRequest("query_too_long", "query is too long", nil)
	}

	// Search
	searchResp, err := s.searcher.Search(ctx, searx.SearchRequest{Query: query})
	if err != nil {
		return Response{}, badGateway("search_upstream_error", "search backend request failed", err)
	}

	// Truncate and renumber
	results := truncateAndRenumber(searchResp.Results, s.opts.MaxSearchResults)

	// No results path — skip LLM
	if len(results) == 0 {
		return Response{
			Query:  query,
			Answer: "No relevant search results were found for the given query.",
			Meta: Meta{
				Model:             s.generator.Model(),
				ResultsConsidered: 0,
				TookMS:            elapsedMS(start),
			},
		}, nil
	}

	// Build prompt sources with capped lengths
	promptSources := make([]sourceForResult, len(results))
	for i, r := range results {
		promptSources[i] = sourceForResult{
			index:   r.Index,
			title:   truncateTitle(r.Title, 200),
			url:     r.URL,
			snippet: truncateSnippet(r.Snippet, 500),
			engine:  r.Engine,
		}
	}

	userPrompt := buildUserPrompt(query, promptSources)

	// Call LLM
	raw, err := s.generator.GenerateJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return Response{}, badGateway("llm_upstream_error", "language model request failed", err)
	}

	// Parse LLM output
	payload, err := parseSynthesisJSON(raw)
	if err != nil {
		code := "llm_invalid_json"
		msg := "language model returned invalid JSON"
		if strings.Contains(err.Error(), "missing answer") {
			code = "llm_invalid_payload"
			msg = "language model returned an invalid payload"
		}
		return Response{}, badGateway(code, msg, err)
	}

	// Sanitize citations
	validCitations := sanitizeCitations(payload.Citations, len(results))

	// Build cited sources
	citedSources := make([]Source, 0, len(validCitations))
	for _, idx := range validCitations {
		for _, r := range results {
			if r.Index == idx {
				citedSources = append(citedSources, Source{
					Index:   r.Index,
					Title:   r.Title,
					URL:     r.URL,
					Snippet: r.Snippet,
					Engine:  r.Engine,
				})
				break
			}
		}
	}

	return Response{
		Query:   query,
		Answer:  payload.Answer,
		Sources: citedSources,
		Meta: Meta{
			Model:             s.generator.Model(),
			ResultsConsidered: len(results),
			TookMS:            elapsedMS(start),
		},
	}, nil
}

func truncateAndRenumber(results []searx.Result, max int) []searx.Result {
	if len(results) > max {
		results = results[:max]
	}
	for i := range results {
		results[i].Index = i + 1
	}
	return results
}

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
