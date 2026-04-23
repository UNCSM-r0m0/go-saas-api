package apikey

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents a user's API key for programmatic access.
type APIKey struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	UserID    uuid.UUID  `json:"user_id"`
	Name      string     `json:"name"`
	KeyHash   string     `json:"-"` // never exposed
	Scopes    []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// IsActive returns true if the key is not revoked and not expired.
func (k *APIKey) IsActive() bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}
	return true
}
