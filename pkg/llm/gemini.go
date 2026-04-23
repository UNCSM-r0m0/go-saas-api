package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GeminiClient implements Client for Google Gemini.
type GeminiClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewGeminiClient creates a new Gemini client.
func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{
		baseURL: "https://generativelanguage.googleapis.com",
		apiKey:  apiKey,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		},
	}
}

// Stream sends a request to Gemini and returns a channel of chunks.
func (c *GeminiClient) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	contents := make([]map[string]any, len(req.Messages))
	for i, m := range req.Messages {
		role := m.Role
		if role == "system" {
			role = "user"
		}
		if role == "assistant" {
			role = "model"
		}
		contents[i] = map[string]any{
			"role": role,
			"parts": []map[string]any{
				{"text": m.Content},
			},
		}
	}

	body, _ := json.Marshal(map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"temperature":      req.Temperature,
			"maxOutputTokens":  req.MaxTokens,
		},
	})

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s", c.baseURL, req.Model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("gemini returned %d", resp.StatusCode)
	}

	ch := make(chan Chunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			// Gemini SSE format: `data: {...}` per line, sometimes prefixed
			if !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "{") {
				// Skip non-JSON lines
				continue
			}

			var parsed struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
					FinishReason string `json:"finishReason"`
				} `json:"candidates"`
			}
			if err := json.Unmarshal([]byte(line), &parsed); err != nil {
				continue
			}
			if len(parsed.Candidates) > 0 {
				content := ""
				for _, part := range parsed.Candidates[0].Content.Parts {
					content += part.Text
				}
				done := parsed.Candidates[0].FinishReason != ""
				select {
				case ch <- Chunk{Content: content, Done: done}:
				case <-ctx.Done():
					return
				}
				if done {
					return
				}
			}
		}
	}()

	return ch, nil
}

// HealthCheck verifies connectivity to Gemini.
func (c *GeminiClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1beta/models?key=%s", c.baseURL, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
