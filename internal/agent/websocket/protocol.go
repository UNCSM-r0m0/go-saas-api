package websocket

import "github.com/google/uuid"

// MessageType defines the kind of WebSocket message.
type MessageType string

const (
	// Client -> Server
	TypeChat  MessageType = "chat"
	TypePing  MessageType = "ping"
	TypeStop  MessageType = "stop"

	// Server -> Client
	TypeChunk MessageType = "chunk"
	TypeDone  MessageType = "done"
	TypeError MessageType = "error"
	TypePong  MessageType = "pong"
)

// Message is the envelope for all WebSocket communications.
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
}
