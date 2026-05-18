package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Result holds the outcome of a rate limit check
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// Limiter implements a sliding window rate limiter using Redis Sorted Sets
type Limiter struct {
	client *redis.Client
	prefix string
}

// NewLimiter creates a new Redis-backed rate limiter
func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{
		client: client,
		prefix: "ratelimit:",
	}
}

// Allow checks if a request is allowed under the given key and limit
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (*Result, error) {
	fullKey := l.prefix + key
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())

	pipe := l.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, fullKey, "0", strconv.FormatInt(windowStart, 10))
	countCmd := pipe.ZCard(ctx, fullKey)
	member := fmt.Sprintf("%d-%d", now, time.Now().UnixNano())
	pipe.ZAdd(ctx, fullKey, redis.Z{Score: float64(now), Member: member})
	pipe.Expire(ctx, fullKey, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("redis pipeline: %w", err)
	}

	count := int(countCmd.Val())
	allowed := count < limit
	remaining := limit - count - 1
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:   allowed,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   time.Now().Add(window),
	}, nil
}
