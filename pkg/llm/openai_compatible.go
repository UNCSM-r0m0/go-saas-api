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

// OpenAICompatibleClient implements Client for OpenAI-compatible APIs.
type OpenAICompatibleClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewOpenAICompatibleClient creates a new OpenAI-compatible client.
func NewOpenAICompatibleClient(baseURL, apiKey string) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		},
	}
}

// Stream sends a request and returns a channel of chunks.
func (c *OpenAICompatibleClient) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"stream":      true,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("provider returned %d", resp.StatusCode)
	}

	ch := make(chan Chunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				select {
				case ch <- Chunk{Done: true}:
				case <-ctx.Done():
					return
				}
				return
			}

			var parsed struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &parsed); err != nil {
				continue
			}
			if len(parsed.Choices) > 0 {
				content := parsed.Choices[0].Delta.Content
				select {
				case ch <- Chunk{Content: content, Done: false}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// HealthCheck verifies connectivity.
func (c *OpenAICompatibleClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
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
