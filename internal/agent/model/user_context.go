package model

import (
	"time"

	"github.com/google/uuid"
)

// ContextSource indicates how a user context item was obtained.
type ContextSource string

const (
	ContextSourceExplicit ContextSource = "explicit"
	ContextSourceInferred ContextSource = "inferred"
)

// UserContextItem represents a persistent fact about a user.
type UserContextItem struct {
	UserID     uuid.UUID     `json:"user_id"`
	Key        string        `json:"key"`
	Value      any           `json:"value"`
	Source     ContextSource `json:"source"`
	Confidence float64       `json:"confidence"`
	UpdatedAt  time.Time     `json:"updated_at"`
}
