package websocket

import "github.com/google/uuid"

type MessageType string

const (
	// Client -> Server
	TypeChat  MessageType = "chat"
	TypePing  MessageType = "ping"
	TypeStop  MessageType = "stop"

	// Server -> Client
	TypeChunk          MessageType = "chunk"
	TypeToolStart      MessageType = "tool_start"
	TypeToolResult     MessageType = "tool_result"
	TypeDone           MessageType = "done"
	TypeError          MessageType = "error"
	TypePong           MessageType = "pong"
	TypeConversationID MessageType = "conversation_id"
)

type Message struct {
	Type           MessageType    `json:"type"`
	MessageID      string         `json:"message_id,omitempty"`
	Content        string         `json:"content,omitempty"`
	ConversationID *uuid.UUID     `json:"conversation_id,omitempty"`
	Model          string         `json:"model,omitempty"`
	FileIDs        []uuid.UUID    `json:"file_ids,omitempty"`
	Done           bool           `json:"done,omitempty"`
	TokensUsed     int            `json:"tokens_used,omitempty"`
	Error          string         `json:"error,omitempty"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolCallID     string         `json:"tool_call_id,omitempty"`
	ToolArgs       map[string]any `json:"tool_args,omitempty"`
}