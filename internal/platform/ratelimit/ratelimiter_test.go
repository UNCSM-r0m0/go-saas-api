package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRedisClient implements the minimal Redis interface needed for testing
// We use a real Redis client via miniredis in integration tests; here we test with a simple mock.
// For unit tests without Redis, we can test the logic directly or skip.

func TestLimiter_Allow(t *testing.T) {
	// This test requires a running Redis instance. Skip if not available.
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	limiter := NewLimiter(client)
	key := "test:user:123"

	// Allow 3 requests
	for i := 0; i < 3; i++ {
		res, err := limiter.Allow(ctx, key, 3, time.Hour)
		require.NoError(t, err)
		assert.True(t, res.Allowed)
		assert.Equal(t, 3, res.Limit)
		assert.Equal(t, 3-i-1, res.Remaining)
	}

	// 4th request should be denied
	res, err := limiter.Allow(ctx, key, 3, time.Hour)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)

	// Cleanup
	_ = client.Del(ctx, "ratelimit:"+key)
}

func TestLimiter_Allow_DifferentKeys(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	limiter := NewLimiter(client)

	res1, err := limiter.Allow(ctx, "user:a", 1, time.Hour)
	require.NoError(t, err)
	assert.True(t, res1.Allowed)

	// Different key should still be allowed
	res2, err := limiter.Allow(ctx, "user:b", 1, time.Hour)
	require.NoError(t, err)
	assert.True(t, res2.Allowed)

	// Cleanup
	_ = client.Del(ctx, "ratelimit:user:a", "ratelimit:user:b")
}
