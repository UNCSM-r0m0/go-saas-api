package llm

import "context"

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is a chat completion request.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

// Chunk is a streamed response chunk.
type Chunk struct {
	Content  string    `json:"content"`
	Done     bool      `json:"done"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

// ToolCall represents a tool invocation from the LLM.
type ToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Client is the unified interface for LLM providers.
type Client interface {
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
	HealthCheck(ctx context.Context) error
}
