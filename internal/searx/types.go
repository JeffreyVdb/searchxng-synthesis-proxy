package searx

// SearchRequest represents a search query.
type SearchRequest struct {
	Query string
}

// Result is a single normalized search result.
type Result struct {
	Index   int
	Title   string
	URL     string
	Snippet string
	Engine  string
}

// SearchResponse contains the normalized search results.
type SearchResponse struct {
	Query   string
	Results []Result
}

// searxWireResponse models only the SearXNG fields we need.
type searxWireResponse struct {
	Query   string `json:"query"`
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Engine  string `json:"engine"`
	} `json:"results"`
}
