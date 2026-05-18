package fileupload

import (
	"time"

	"github.com/google/uuid"
)

// Upload represents a stored file.
type Upload struct {
	ID             uuid.UUID      `json:"id"`
	UserID         uuid.UUID      `json:"user_id"`
	ConversationID *uuid.UUID     `json:"conversation_id,omitempty"`
	Name           string         `json:"name"`
	OriginalName   string         `json:"original_name"`
	ContentType    string         `json:"content_type"`
	SizeBytes      int64          `json:"size_bytes"`
	StoragePath    string         `json:"-"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
}
