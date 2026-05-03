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

// KimiAnthropicClient implements Client for Kimi's Anthropic-compatible API.
// Endpoint: https://api.kimi.com/coding/v1/messages
type KimiAnthropicClient struct {
	apiKey string
	client *http.Client
}

// NewKimiAnthropicClient creates a new Kimi Anthropic-compatible client.
func NewKimiAnthropicClient(apiKey string) *KimiAnthropicClient {
	return &KimiAnthropicClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		},
	}
}

// anthropicMessage represents the request body format.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicRequest represents the API request payload.
type anthropicRequest struct {
	Model     string              `json:"model"`
	Messages  []anthropicMessage  `json:"messages"`
	MaxTokens int                 `json:"max_tokens"`
	Stream    bool                `json:"stream,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
}

// anthropicResponse represents the non-streaming API response.
type anthropicResponse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Role     string `json:"role"`
	Model    string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content  []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Stream sends a request and returns a channel of chunks.
func (c *KimiAnthropicClient) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	payload := anthropicRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
		Temperature: req.Temperature,
	}
	if payload.MaxTokens == 0 {
		payload.MaxTokens = 4096
	}
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.kimi.com/coding/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)

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

			var parsed struct {
				Type  string `json:"type"`
				Index int    `json:"index"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &parsed); err != nil {
				continue
			}

			if parsed.Type == "content_block_delta" && parsed.Delta.Text != "" {
				select {
				case ch <- Chunk{Content: parsed.Delta.Text}:
				case <-ctx.Done():
					return
				}
			}

			if parsed.Type == "message_stop" {
				select {
				case ch <- Chunk{Done: true}:
				case <-ctx.Done():
				}
				return
			}
		}
		// If scanner finishes without message_stop, send done
		select {
		case ch <- Chunk{Done: true}:
		case <-ctx.Done():
		}
	}()

	return ch, nil
}

// Complete sends a non-streaming request and returns the full response text.
func (c *KimiAnthropicClient) Complete(ctx context.Context, req Request) (string, error) {
	payload := anthropicRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
		Temperature: req.Temperature,
	}
	if payload.MaxTokens == 0 {
		payload.MaxTokens = 4096
	}
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.kimi.com/coding/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("provider returned %d", resp.StatusCode)
	}

	var parsed anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	var result strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}
	return result.String(), nil
}

// HealthCheck verifies connectivity by calling the models endpoint.
func (c *KimiAnthropicClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.kimi.com/coding/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.apiKey)
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
