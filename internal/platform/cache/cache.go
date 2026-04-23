package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is a Redis-backed cache wrapper.
type Cache struct {
	client *redis.Client
	prefix string
}

// NewCache creates a new cache instance.
func NewCache(client *redis.Client, prefix string) *Cache {
	return &Cache{client: client, prefix: prefix}
}

func (c *Cache) key(k string) string {
	return c.prefix + ":" + k
}

// Get retrieves a value and unmarshals it into dest.
func (c *Cache) Get(ctx context.Context, key string, dest any) error {
	data, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Set marshals value and stores it with TTL.
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.client.Set(ctx, c.key(key), data, ttl).Err()
}

// Delete removes a key from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.key(key)).Err()
}

// Invalidate removes keys matching a pattern.
func (c *Cache) Invalidate(ctx context.Context, pattern string) error {
	keys, err := c.client.Keys(ctx, c.key(pattern)).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}
	return nil
}
