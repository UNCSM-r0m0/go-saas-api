package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// PasswordResetTokenStore defines the interface for password reset token persistence
type PasswordResetTokenStore interface {
	Save(ctx context.Context, token, userID string, expiration time.Duration) error
	Get(ctx context.Context, token string) (string, error) // returns userID
	Delete(ctx context.Context, token string) error
}

// RedisPasswordResetTokenStore implements PasswordResetTokenStore using Redis
type RedisPasswordResetTokenStore struct {
	client *redis.Client
	prefix string
}

// NewRedisPasswordResetTokenStore creates a new Redis-backed password reset token store
func NewRedisPasswordResetTokenStore(client *redis.Client) *RedisPasswordResetTokenStore {
	return &RedisPasswordResetTokenStore{
		client: client,
		prefix: "pwdreset:",
	}
}

// Save stores a password reset token with expiration
func (s *RedisPasswordResetTokenStore) Save(ctx context.Context, token, userID string, expiration time.Duration) error {
	key := s.prefix + token
	return s.client.Set(ctx, key, userID, expiration).Err()
}

// Get retrieves the user ID associated with a password reset token
func (s *RedisPasswordResetTokenStore) Get(ctx context.Context, token string) (string, error) {
	key := s.prefix + token
	return s.client.Get(ctx, key).Result()
}

// Delete removes a password reset token
func (s *RedisPasswordResetTokenStore) Delete(ctx context.Context, token string) error {
	key := s.prefix + token
	return s.client.Del(ctx, key).Err()
}

// Ensure RedisPasswordResetTokenStore implements PasswordResetTokenStore
var _ PasswordResetTokenStore = (*RedisPasswordResetTokenStore)(nil)

// GeneratePasswordResetToken generates a cryptographically secure random token
func GeneratePasswordResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
