package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisOAuthStateStore implements OAuthStateStore using Redis
type RedisOAuthStateStore struct {
	client *redis.Client
	prefix string
}

// NewRedisOAuthStateStore creates a new Redis-backed OAuth state store
func NewRedisOAuthStateStore(client *redis.Client) *RedisOAuthStateStore {
	return &RedisOAuthStateStore{
		client: client,
		prefix: "oauth_state:",
	}
}

// SaveState stores an OAuth state token with expiration
func (s *RedisOAuthStateStore) SaveState(ctx context.Context, state string, expiration time.Duration) error {
	return s.client.Set(ctx, s.prefix+state, "1", expiration).Err()
}

// ValidateState checks if a state token exists and removes it (one-time use)
func (s *RedisOAuthStateStore) ValidateState(ctx context.Context, state string) (bool, error) {
	key := s.prefix + state
	val, err := s.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

// Ensure RedisOAuthStateStore implements OAuthStateStore
var _ OAuthStateStore = (*RedisOAuthStateStore)(nil)
