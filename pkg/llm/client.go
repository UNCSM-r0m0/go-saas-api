package llm

import "context"

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type FunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolDefinition struct {
	Type     string         `json:"type"`
	Function FunctionSchema `json:"function"`
}

type Request struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream"`
}

type Chunk struct {
	Content  string    `json:"content"`
	Done     bool      `json:"done"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Event    string    `json:"event,omitempty"`
	ToolName string    `json:"tool_name,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type Client interface {
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
	Complete(ctx context.Context, req Request) (string, error)
	HealthCheck(ctx context.Context) error
}
