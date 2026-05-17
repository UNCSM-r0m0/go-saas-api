package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebReaderTool reads the content of a specific web page using Jina AI Reader.
type WebReaderTool struct {
	client *http.Client
}

// NewWebReaderTool creates a new web_reader tool.
func NewWebReaderTool() *WebReaderTool {
	return NewWebReaderToolWithClient(&http.Client{Timeout: 20 * time.Second})
}

// NewWebReaderToolWithClient creates a new web_reader tool with a custom HTTP client.
func NewWebReaderToolWithClient(client *http.Client) *WebReaderTool {
	return &WebReaderTool{client: client}
}

// Name returns the tool name.
func (w *WebReaderTool) Name() string { return "web_reader" }

// Description returns the tool description.
func (w *WebReaderTool) Description() string {
	return "Read and extract clean text content from a specific web page URL using Jina AI Reader. Use when the user shares a URL or when you need detailed content from a specific page. Args: url (string)."
}

// Schema returns the JSON Schema for the tool's parameters.
func (w *WebReaderTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The full URL of the web page to read (e.g., https://example.com/article)",
			},
		},
		"required": []string{"url"},
	}
}

// Execute reads the web page using Jina AI Reader and returns the clean content.
func (w *WebReaderTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	pageURL, ok := args["url"].(string)
	if !ok || pageURL == "" {
		return Result{Error: "missing or invalid 'url' argument"}, fmt.Errorf("missing url")
	}

	// Ensure URL has scheme
	if !strings.HasPrefix(pageURL, "http://") && !strings.HasPrefix(pageURL, "https://") {
		pageURL = "https://" + pageURL
	}

	// Jina AI Reader endpoint: https://r.jina.ai/http://URL or https://r.jina.ai/http://URL
	jinaURL := "https://r.jina.ai/http://" + strings.TrimPrefix(pageURL, "https://")
	if strings.HasPrefix(pageURL, "https://") {
		jinaURL = "https://r.jina.ai/https://" + strings.TrimPrefix(pageURL, "https://")
	} else if strings.HasPrefix(pageURL, "http://") {
		jinaURL = "https://r.jina.ai/http://" + strings.TrimPrefix(pageURL, "http://")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jinaURL, nil)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{Error: fmt.Sprintf("Jina Reader returned status %d", resp.StatusCode)}, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	content := string(body)
	if strings.TrimSpace(content) == "" {
		return Result{Error: "page content is empty"}, fmt.Errorf("empty content")
	}

	// Truncate very long content
	if len(content) > 15000 {
		content = content[:15000] + "\n\n...[content truncated]"
	}

	return Result{Content: fmt.Sprintf("Content from %s:\n\n%s", pageURL, content)}, nil
}

// cleanURL removes trailing slashes and normalizes the URL for display.
func cleanURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
