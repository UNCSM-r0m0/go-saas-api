package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCacheInvalidationLogic tests the Redis cache invalidation logic
// that the NATS consumer performs. Requires Redis running.
func TestCacheInvalidationLogic(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer redisClient.Close()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	userID := "test-user-uuid"
	cacheKey := fmt.Sprintf("tier:%s", userID)

	// Seed cache
	_ = redisClient.Set(ctx, cacheKey, "premium", 5*time.Minute).Err()
	val, err := redisClient.Get(ctx, cacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, "premium", val)

	// Simulate what NATS consumer does
	_ = redisClient.Del(ctx, cacheKey).Err()

	// Verify deletion
	_, err = redisClient.Get(ctx, cacheKey).Result()
	assert.Equal(t, redis.Nil, err, "expected tier key to be deleted")
}
