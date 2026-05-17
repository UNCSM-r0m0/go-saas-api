package tools

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSearchClient is a test double for SearchClient.
type mockSearchClient struct {
	results []SearchResult
	err     error
}

func (m *mockSearchClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
	return m.results, m.err
}

func TestWebSearchTool_Name(t *testing.T) {
	tool := NewWebSearchTool()
	assert.Equal(t, "web_search", tool.Name())
}

func TestWebSearchTool_Description(t *testing.T) {
	tool := NewWebSearchTool()
	assert.Equal(t, "Search the web for information. Args: query (string), num_results (int, default 5).", tool.Description())
}

func TestWebSearchTool_Schema(t *testing.T) {
	tool := NewWebSearchTool()
	schema := tool.Schema()

	assert.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "query")
	assert.Contains(t, props, "num_results")

	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "query")
}

func TestWebSearchTool_Execute_MissingQuery(t *testing.T) {
	tool := NewWebSearchTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, result.Error, "missing or invalid")
}

func TestWebSearchTool_Execute_EmptyQuery(t *testing.T) {
	tool := NewWebSearchTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"query": ""})
	assert.Error(t, err)
	assert.Contains(t, result.Error, "missing or invalid")
}

func TestWebSearchTool_Execute_Success(t *testing.T) {
	mock := &mockSearchClient{
		results: []SearchResult{
			{Title: "Go Programming", URL: "https://go.dev", Snippet: "The Go programming language."},
			{Title: "Go Docs", URL: "https://pkg.go.dev", Snippet: "Go package documentation."},
		},
	}
	tool := NewWebSearchToolWithClient(mock)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"query": "golang", "num_results": 2.0})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Content, "Go Programming")
	assert.Contains(t, result.Content, "https://go.dev")
	assert.Contains(t, result.Content, "Go Docs")
	assert.Contains(t, result.Content, "Search results for")
}

func TestWebSearchTool_Execute_DefaultNumResults(t *testing.T) {
	mock := &mockSearchClient{
		results: []SearchResult{
			{Title: "Result 1", URL: "https://example.com/1", Snippet: "Snippet 1"},
			{Title: "Result 2", URL: "https://example.com/2", Snippet: "Snippet 2"},
			{Title: "Result 3", URL: "https://example.com/3", Snippet: "Snippet 3"},
			{Title: "Result 4", URL: "https://example.com/4", Snippet: "Snippet 4"},
			{Title: "Result 5", URL: "https://example.com/5", Snippet: "Snippet 5"},
			{Title: "Result 6", URL: "https://example.com/6", Snippet: "Snippet 6"},
		},
	}
	tool := NewWebSearchToolWithClient(mock)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"query": "test"})
	require.NoError(t, err)
	// Default is 5 results
	lines := 0
	for _, ch := range result.Content {
		if ch == '\n' {
			lines++
		}
	}
	assert.Equal(t, 6, lines) // 1 header + 5 results
}

func TestWebSearchTool_Execute_MaxResultsCap(t *testing.T) {
	mock := &mockSearchClient{
		results: make([]SearchResult, 15),
	}
	for i := range 15 {
		mock.results[i] = SearchResult{
			Title:   fmt.Sprintf("Result %d", i+1),
			URL:     fmt.Sprintf("https://example.com/%d", i+1),
			Snippet: fmt.Sprintf("Snippet %d", i+1),
		}
	}
	tool := NewWebSearchToolWithClient(mock)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"query": "test", "num_results": 20.0})
	require.NoError(t, err)
	// Should be capped at 10
	lines := 0
	for _, ch := range result.Content {
		if ch == '\n' {
			lines++
		}
	}
	assert.Equal(t, 11, lines) // 1 header + 10 results
}

func TestWebSearchTool_Execute_ClientError(t *testing.T) {
	mock := &mockSearchClient{err: fmt.Errorf("network error")}
	tool := NewWebSearchToolWithClient(mock)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"query": "test"})
	assert.Error(t, err)
	assert.Contains(t, result.Error, "network error")
}

func TestWebSearchTool_Execute_NoResults(t *testing.T) {
	mock := &mockSearchClient{results: []SearchResult{}}
	tool := NewWebSearchToolWithClient(mock)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"query": "xyznonexistent"})
	require.NoError(t, err)
	assert.Contains(t, result.Content, "No results found")
}

func TestWebSearchTool_Execute_NumResultsAsInt(t *testing.T) {
	mock := &mockSearchClient{
		results: []SearchResult{
			{Title: "Result 1", URL: "https://example.com/1", Snippet: "Snippet 1"},
			{Title: "Result 2", URL: "https://example.com/2", Snippet: "Snippet 2"},
			{Title: "Result 3", URL: "https://example.com/3", Snippet: "Snippet 3"},
		},
	}
	tool := NewWebSearchToolWithClient(mock)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"query": "test", "num_results": 2})
	require.NoError(t, err)
	lines := 0
	for _, ch := range result.Content {
		if ch == '\n' {
			lines++
		}
	}
	assert.Equal(t, 3, lines) // 1 header + 2 results
}

func TestParseDuckDuckGoResults(t *testing.T) {
	html := `
		<div class="result results_links results_links_deep web-result">
			<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev">Go Programming Language</a>
			<a class="result__url" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev">go.dev</a>
			<div class="result__snippet">The Go programming language is an open source project.</div>
		</div>
		<div class="result results_links results_links_deep web-result">
			<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fpkg.go.dev">Go Packages</a>
			<a class="result__url" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fpkg.go.dev">pkg.go.dev</a>
			<div class="result__snippet">Package documentation for Go.</div>
		</div>
	`

	results := parseDuckDuckGoResults(html)
	require.Len(t, results, 2)

	assert.Equal(t, "Go Programming Language", results[0].Title)
	assert.Equal(t, "https://go.dev", results[0].URL)
	assert.Equal(t, "The Go programming language is an open source project.", results[0].Snippet)

	assert.Equal(t, "Go Packages", results[1].Title)
	assert.Equal(t, "https://pkg.go.dev", results[1].URL)
	assert.Equal(t, "Package documentation for Go.", results[1].Snippet)
}

func TestParseDuckDuckGoResults_Empty(t *testing.T) {
	results := parseDuckDuckGoResults("")
	assert.Empty(t, results)
}

func TestCleanDuckDuckGoURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev",
			expected: "https://go.dev",
		},
		{
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			input:    "//duckduckgo.com/l/?kh=https%3A%2F%2Fexample.org",
			expected: "https://example.org",
		},
		{
			input:    "//duckduckgo.com/l/?",
			expected: "//duckduckgo.com/l/?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := cleanDuckDuckGoURL(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestCleanHTML(t *testing.T) {
	input := "The <b>Go</b> language &amp; its features"
	expected := "The Go language & its features"
	assert.Equal(t, expected, cleanHTML(input))
}
