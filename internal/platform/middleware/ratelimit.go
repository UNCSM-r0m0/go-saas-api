package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	ResolveTier(ctx context.Context, userID string) (tier string, err error)
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
		tier := "registered" // default
		if resolver != nil {
			resolvedTier, err := resolver.ResolveTier(c.Request.Context(), fmt.Sprintf("%v", uid))
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

	// Anonymous users: use anonymousId from body when present, fallback to IP
	anonID := extractAnonymousIDFromContext(c)
	if anonID != "" {
		return fmt.Sprintf("anon:%s", anonID), cfg.FreeMessageLimit
	}

	return fmt.Sprintf("ip:%s", c.ClientIP()), cfg.FreeMessageLimit
}

// SandboxRateLimit creates a strict rate limiter for sandbox endpoints.
// Enforces cfg.SandboxRateLimit req/min per user and 1 req/min per anonymousId.
func SandboxRateLimit(limiter *ratelimit.Limiter, cfg *config.Config, log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, limit := resolveSandboxLimit(c, cfg)

		result, err := limiter.Allow(c.Request.Context(), key, limit, time.Minute)
		if err != nil {
			log.Error("sandbox rate limit check failed", logger.Error(err))
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetAt.Unix()))

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "sandbox rate limit exceeded",
				"limit":       result.Limit,
				"reset_at":    result.ResetAt.Unix(),
				"retry_after": fmt.Sprintf("%d", int(time.Until(result.ResetAt).Seconds())),
			})
			return
		}

		c.Next()
	}
}

func resolveSandboxLimit(c *gin.Context, cfg *config.Config) (string, int) {
	if uid, exists := c.Get("user_id"); exists && uid != "" {
		return fmt.Sprintf("sandbox:user:%s", uid), cfg.SandboxRateLimit
	}
	anonID := extractAnonymousIDFromContext(c)
	if anonID != "" {
		return fmt.Sprintf("sandbox:anon:%s", anonID), 1
	}
	return fmt.Sprintf("sandbox:ip:%s", c.ClientIP()), 1
}

// extractAnonymousIDFromContext extracts anonymousId from the request body if present.
func extractAnonymousIDFromContext(c *gin.Context) string {
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		body, err := io.ReadAll(c.Request.Body)
		if err == nil && len(body) > 0 {
			var payload struct {
				AnonymousID string `json:"anonymousId"`
			}
			if json.Unmarshal(body, &payload) == nil && payload.AnonymousID != "" {
				// Restore body so downstream handlers can read it again
				c.Request.Body = io.NopCloser(bytes.NewReader(body))
				return payload.AnonymousID
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
	}
	return ""
}
