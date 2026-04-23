package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaClient implements Client for Ollama.
type OllamaClient struct {
	baseURL string
	client  *http.Client
}

// NewOllamaClient creates a new Ollama client.
func NewOllamaClient(baseURL string) *OllamaClient {
	return &OllamaClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		},
	}
}

// Stream sends a request to Ollama and returns a channel of chunks.
func (c *OllamaClient) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	body, _ := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
		"options": map[string]any{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(body))
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
		return nil, fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	ch := make(chan Chunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			var parsed struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := json.Unmarshal(line, &parsed); err != nil {
				continue
			}
			select {
			case ch <- Chunk{Content: parsed.Message.Content, Done: parsed.Done}:
			case <-ctx.Done():
				return
			}
			if parsed.Done {
				return
			}
		}
	}()

	return ch, nil
}

// HealthCheck verifies connectivity to Ollama.
func (c *OllamaClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
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
