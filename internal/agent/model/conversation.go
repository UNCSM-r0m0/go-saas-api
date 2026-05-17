package model

import (
	"time"

	"github.com/google/uuid"
)

// ConversationStatus represents the state of a conversation.
type ConversationStatus string

const (
	ConversationActive   ConversationStatus = "active"
	ConversationArchived ConversationStatus = "archived"
	ConversationDeleted  ConversationStatus = "deleted"
)

// Conversation is a chat thread between a user and an agent.
type Conversation struct {
	ID        uuid.UUID          `json:"id"`
	UserID    uuid.UUID          `json:"user_id"`
	Title     string             `json:"title"`
	AgentID   *uuid.UUID         `json:"agent_id,omitempty"`
	Status    ConversationStatus `json:"status"`
	Metadata  map[string]any     `json:"metadata"`
	Messages  []Message          `json:"messages,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}
