package model

import (
	"time"

	"github.com/google/uuid"
)

// Artifact is a generated file attached to a conversation.
type Artifact struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	ConversationID uuid.UUID  `json:"conversation_id"`
	MessageID      *uuid.UUID `json:"message_id,omitempty"`
	Name           string     `json:"name"`
	Type           string     `json:"type"`
	Language       string     `json:"language,omitempty"`
	Content        string     `json:"content"`
	Version        int        `json:"version"`
	IsDeleted      bool       `json:"is_deleted"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
