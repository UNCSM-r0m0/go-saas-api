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

type TierResolver interface {
	ResolveTier(ctx context.Context, userID string) (tier string, err error)
}

func RateLimit(limiter *ratelimit.Limiter, cfg *config.Config, log logger.Logger, resolver TierResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/health" || strings.HasPrefix(path, "/api/v1/auth") {
			c.Next()
			return
		}

		key, limit := resolveLimit(c, cfg, resolver)

		result, err := limiter.Allow(c.Request.Context(), key, limit, 30*24*time.Hour)
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
	if uid, exists := c.Get("user_id"); exists && uid != "" {
		tier := "registered"
		if resolver != nil {
			resolvedTier, err := resolver.ResolveTier(c.Request.Context(), fmt.Sprintf("%v", uid))
			if err == nil && resolvedTier != "" {
				tier = resolvedTier
			}
		}
		switch tier {
		case "premium":
			return fmt.Sprintf("user:%s", uid), cfg.PremiumMessageLimit
		default:
			return fmt.Sprintf("user:%s", uid), cfg.RegisteredMessageLimit
		}
	}

	return "anonymous:blocked", 0
}

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
	return fmt.Sprintf("sandbox:ip:%s", c.ClientIP()), 1
}
