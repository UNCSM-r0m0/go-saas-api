package model

import (
	"time"

	"github.com/google/uuid"
)

// ArtifactFile represents a single file within an artifact project.
type ArtifactFile struct {
	Path     string `json:"path"`
	Language string `json:"language,omitempty"`
	Content  string `json:"content"`
}

// ArtifactProject is a bundle of related files (e.g., a website).
type ArtifactProject struct {
	ID             uuid.UUID      `json:"id"`
	ConversationID uuid.UUID      `json:"conversation_id"`
	Type           string         `json:"type"`
	Files          []ArtifactFile `json:"files"`
	EntryFile      string         `json:"entry_file"`
	Version        int            `json:"version"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// ArtifactResponse is the metadata returned alongside a chat message.
type ArtifactResponse struct {
	ArtifactID   string `json:"artifactId,omitempty"`
	ArtifactType string `json:"artifactType,omitempty"`
}
