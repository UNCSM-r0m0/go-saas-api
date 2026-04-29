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

// accumulatedToolCall tracks a tool call being built across SSE chunks.
type accumulatedToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

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
	payload := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"stream":      true,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	body, _ := json.Marshal(payload)

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

		accumulated := make(map[int]*accumulatedToolCall)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				emitFinal(ch, ctx, accumulated)
				return
			}

			var parsed struct {
				Choices []struct {
					Delta struct {
						Content   string `json:"content"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Type     string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &parsed); err != nil {
				continue
			}
			if len(parsed.Choices) == 0 {
				continue
			}

			choice := parsed.Choices[0]

			// Emit content chunks
			if choice.Delta.Content != "" {
				select {
				case ch <- Chunk{Content: choice.Delta.Content}:
				case <-ctx.Done():
					return
				}
			}

			// Accumulate tool calls
			for _, tc := range choice.Delta.ToolCalls {
				if _, ok := accumulated[tc.Index]; !ok {
					accumulated[tc.Index] = &accumulatedToolCall{id: tc.ID}
				}
				acc := accumulated[tc.Index]
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				acc.arguments.WriteString(tc.Function.Arguments)
			}

			// Handle finish reason
			if choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" {
				emitFinal(ch, ctx, accumulated)
				return
			}
		}
	}()

	return ch, nil
}

// emitFinal sends the final chunk(s) including any accumulated tool calls.
func emitFinal(ch chan Chunk, ctx context.Context, accumulated map[int]*accumulatedToolCall) {
	if len(accumulated) == 0 {
		select {
		case ch <- Chunk{Done: true}:
		case <-ctx.Done():
		}
		return
	}
	// Emit the first accumulated tool call as the final chunk
	for _, acc := range accumulated {
		var args map[string]any
		_ = json.Unmarshal([]byte(acc.arguments.String()), &args)
		select {
		case ch <- Chunk{
			Done: true,
			ToolCall: &ToolCall{
				Name:      acc.name,
				Arguments: args,
			},
		}:
		case <-ctx.Done():
		}
		return
	}
}

// Complete sends a non-streaming request and returns the full response text.
func (c *OpenAICompatibleClient) Complete(ctx context.Context, req Request) (string, error) {
	payload := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"stream":      false,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("provider returned %d", resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return parsed.Choices[0].Message.Content, nil
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
