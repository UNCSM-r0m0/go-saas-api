package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/ratelimit"
)

// TierResolver resolves the rate-limit tier for an authenticated user.
type TierResolver interface {
	ResolveTier(ctx context.Context, tenantID, userID string) (tier string, err error)
}

// RateLimit creates a middleware that enforces rate limits per tier.
func RateLimit(limiter *ratelimit.Limiter, cfg *config.Config, log logger.Logger, resolver TierResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting for health checks and auth endpoints
		path := c.Request.URL.Path
		if path == "/health" || strings.HasPrefix(path, "/api/v1/auth") {
			c.Next()
			return
		}

		key, limit := resolveLimit(c, cfg, resolver)

		result, err := limiter.Allow(c.Request.Context(), key, limit, 24*time.Hour)
		if err != nil {
			log.Error("rate limit check failed", logger.Error(err))
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetAt.Unix()))

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"limit":       result.Limit,
				"reset_at":    result.ResetAt.Unix(),
				"retry_after": fmt.Sprintf("%d", int(time.Until(result.ResetAt).Seconds())),
			})
			return
		}

		c.Next()
	}
}

func resolveLimit(c *gin.Context, cfg *config.Config, resolver TierResolver) (string, int) {
	// Authenticated users: use user_id as key
	if uid, exists := c.Get("user_id"); exists && uid != "" {
		tenantID, _ := c.Get("tenant_id")
		tier := "registered" // default
		if resolver != nil {
			resolvedTier, err := resolver.ResolveTier(c.Request.Context(), fmt.Sprintf("%v", tenantID), fmt.Sprintf("%v", uid))
			if err == nil && resolvedTier != "" {
				tier = resolvedTier
			}
		}
		switch tier {
		case "premium":
			return fmt.Sprintf("user:%s", uid), cfg.PremiumMessageLimit
		case "free":
			return fmt.Sprintf("user:%s", uid), cfg.FreeMessageLimit
		default:
			return fmt.Sprintf("user:%s", uid), cfg.RegisteredMessageLimit
		}
	}

	// Anonymous users: use client IP as key
	return fmt.Sprintf("ip:%s", c.ClientIP()), cfg.FreeMessageLimit
}
