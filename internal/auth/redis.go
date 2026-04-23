package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRefreshTokenStore implements RefreshTokenStore using Redis
type RedisRefreshTokenStore struct {
	client *redis.Client
	prefix string
}

// NewRedisRefreshTokenStore creates a new Redis-backed refresh token store
func NewRedisRefreshTokenStore(client *redis.Client) *RedisRefreshTokenStore {
	return &RedisRefreshTokenStore{
		client: client,
		prefix: "refresh:",
	}
}

// Save stores a refresh token with expiration
func (s *RedisRefreshTokenStore) Save(ctx context.Context, userID, token string, expiration time.Duration) error {
	key := s.prefix + token
	return s.client.Set(ctx, key, userID, expiration).Err()
}

// Get retrieves the user ID associated with a refresh token
func (s *RedisRefreshTokenStore) Get(ctx context.Context, token string) (string, error) {
	key := s.prefix + token
	return s.client.Get(ctx, key).Result()
}

// Delete removes a refresh token
func (s *RedisRefreshTokenStore) Delete(ctx context.Context, token string) error {
	key := s.prefix + token
	return s.client.Del(ctx, key).Err()
}

// Ensure RedisRefreshTokenStore implements RefreshTokenStore
var _ RefreshTokenStore = (*RedisRefreshTokenStore)(nil)
