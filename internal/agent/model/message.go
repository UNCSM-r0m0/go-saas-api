package model

import (
	"time"

	"github.com/google/uuid"
)

// MessageRole represents who sent a message.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// ToolCall represents an LLM tool invocation.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Message is a single turn in a conversation.
type Message struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	ConversationID uuid.UUID  `json:"conversation_id"`
	Role           MessageRole `json:"role"`
	Content        string     `json:"content"`
	ToolCalls      []ToolCall `json:"tool_calls,omitempty"`
	Model          string     `json:"model,omitempty"`
	TokensInput    int        `json:"tokens_input,omitempty"`
	TokensOutput   int        `json:"tokens_output,omitempty"`
	LatencyMs      int        `json:"latency_ms,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
