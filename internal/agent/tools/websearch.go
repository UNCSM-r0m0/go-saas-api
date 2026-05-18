package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// SearchClient fetches web search results.
type SearchClient interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

// SearchResult represents a single web search result.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// WebSearchTool searches the web using DuckDuckGo.
type WebSearchTool struct {
	client SearchClient
}

// NewWebSearchTool creates a new web_search tool with the default HTTP client.
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		client: &duckDuckGoClient{
			client: &http.Client{Timeout: 15 * time.Second},
		},
	}
}

// NewWebSearchToolWithClient creates a new web_search tool with a custom client (useful for testing).
func NewWebSearchToolWithClient(client SearchClient) *WebSearchTool {
	return &WebSearchTool{client: client}
}

// Name returns the tool name.
func (w *WebSearchTool) Name() string { return "web_search" }

// Description returns the tool description.
func (w *WebSearchTool) Description() string {
	return "Search the web for information. Args: query (string), num_results (int, default 5)."
}

// Schema returns the JSON Schema for the tool's parameters.
func (w *WebSearchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query string",
			},
			"num_results": map[string]any{
				"type":        "integer",
				"description": "Number of results to return (default 5, max 10)",
				"minimum":     1,
				"maximum":     10,
			},
		},
		"required": []string{"query"},
	}
}

// Execute performs a web search and returns formatted results.
func (w *WebSearchTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return Result{Error: "missing or invalid 'query' argument"}, fmt.Errorf("missing query")
	}

	numResults := 5
	if n, ok := args["num_results"].(float64); ok {
		numResults = int(n)
	} else if n, ok := args["num_results"].(int); ok {
		numResults = n
	}
	if numResults < 1 {
		numResults = 5
	}
	if numResults > 10 {
		numResults = 10
	}

	results, err := w.client.Search(ctx, query)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	if len(results) == 0 {
		return Result{Content: fmt.Sprintf("No results found for %q.", query)}, nil
	}

	if len(results) > numResults {
		results = results[:numResults]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for %q:\n", query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s) - %s\n", i+1, r.Title, r.URL, r.Snippet))
	}

	return Result{Content: sb.String()}, nil
}

// duckDuckGoClient is a SearchClient that uses DuckDuckGo HTML search.
type duckDuckGoClient struct {
	client *http.Client
}

// Search queries DuckDuckGo and parses the HTML results.
func (c *duckDuckGoClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseDuckDuckGoResults(string(body)), nil
}

// parseDuckDuckGoResults extracts search results from DuckDuckGo HTML.
func parseDuckDuckGoResults(html string) []SearchResult {
	var results []SearchResult

	// DuckDuckGo HTML structure uses result blocks with class "web-result" or "result"
	// We'll split by result blocks and parse each one.
	resultBlocks := splitResultBlocks(html)

	for _, block := range resultBlocks {
		title := extractTagContent(block, `<a[^>]*class="result__a"[^>]*>([^<]*)</a>`)
		rawURL := extractAttr(block, `<a[^>]*class="result__a"[^>]*href="([^"]*)"`)
		snippet := extractTagContent(block, `<a[^>]*class="result__snippet"[^>]*>([^<]*)</a>`)

		if snippet == "" {
			snippet = extractTagContent(block, `<div[^>]*class="result__snippet"[^>]*>([^<]*)</div>`)
		}
		if snippet == "" {
			snippet = extractTagContent(block, `<div[^>]*class="result__snippet"[^>]*>(.*?)</div>`)
		}

		if title == "" || rawURL == "" {
			continue
		}

		// DuckDuckGo redirects through their own URL
		actualURL := cleanDuckDuckGoURL(rawURL)

		results = append(results, SearchResult{
			Title:   strings.TrimSpace(title),
			URL:     actualURL,
			Snippet: strings.TrimSpace(snippet),
		})
	}

	return results
}

// splitResultBlocks splits DuckDuckGo HTML into individual result blocks.
func splitResultBlocks(html string) []string {
	// DuckDuckGo uses outer divs with class "result" AND "results_links" for each search result.
	// We must be specific to avoid matching inner elements like "result__snippet".
	re := regexp.MustCompile(`<div[^>]*class="[^"]*\bresult\b[^"]*\bresults_links\b[^"]*"[^>]*>`)
	indices := re.FindAllStringIndex(html, -1)
	if len(indices) == 0 {
		return nil
	}

	var blocks []string
	for i, idx := range indices {
		start := idx[0]
		var end int
		if i+1 < len(indices) {
			end = indices[i+1][0]
		} else {
			end = len(html)
		}
		blocks = append(blocks, html[start:end])
	}
	return blocks
}

// extractTagContent extracts the content between HTML tags using a regex.
func extractTagContent(html, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return cleanHTML(matches[1])
	}
	return ""
}

// extractAttr extracts an attribute value from an HTML tag.
func extractAttr(html, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// cleanDuckDuckGoURL resolves DuckDuckGo redirect URLs to actual URLs.
func cleanDuckDuckGoURL(rawURL string) string {
	if strings.HasPrefix(rawURL, "//duckduckgo.com/l/?") {
		u, err := url.Parse("https:" + rawURL)
		if err != nil {
			return rawURL
		}
		if actual := u.Query().Get("uddg"); actual != "" {
			return actual
		}
		if actual := u.Query().Get("kh"); actual != "" {
			return actual
		}
	}
	return rawURL
}

// cleanHTML removes HTML tags and decodes common entities from a string.
func cleanHTML(s string) string {
	// Remove remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	s = tagRe.ReplaceAllString(s, "")

	// Decode common entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")

	return s
}
