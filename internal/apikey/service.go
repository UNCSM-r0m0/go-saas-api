package apikey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service provides business logic for API key management.
type Service struct {
	store Store
}

// NewService creates a new API key service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// GenerateKey creates a new API key, returns the plain key (once) and the persisted record.
func (s *Service) GenerateKey(ctx context.Context, userID uuid.UUID, name string, expiresAt *time.Time) (plainKey string, key *APIKey, err error) {
	plainKey = generatePlainKey()
	hash := hashKey(plainKey)

	key = &APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		KeyHash:   hash,
		Scopes:    []string{"api"},
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.store.Create(ctx, key); err != nil {
		return "", nil, fmt.Errorf("create api key: %w", err)
	}
	return plainKey, key, nil
}

// ListKeys returns all active API keys for a user.
func (s *Service) ListKeys(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	keys, err := s.store.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	return keys, nil
}

// RevokeKey revokes an API key.
func (s *Service) RevokeKey(ctx context.Context, id uuid.UUID) error {
	if err := s.store.Revoke(ctx, id); err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	return nil
}

// ValidateKey checks if an API key is valid and returns the associated key record.
func (s *Service) ValidateKey(ctx context.Context, plainKey string) (*APIKey, error) {
	hash := hashKey(plainKey)
	key, err := s.store.GetByKeyHash(ctx, hash)
	if err != nil || key == nil {
		return nil, fmt.Errorf("invalid api key")
	}
	if !key.IsActive() {
		return nil, fmt.Errorf("api key revoked or expired")
	}
	_ = s.store.UpdateLastUsed(ctx, key.ID)
	return key, nil
}

func generatePlainKey() string {
	return "sk_" + uuid.New().String() + uuid.New().String()
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
