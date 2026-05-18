package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit_SkipsAuthAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	cfg := &config.Config{
		RegisteredMessageLimit: 10,
		PremiumMessageLimit:    100,
	}
	log := logger.New("error")
	limiter := ratelimit.NewLimiter(client)

	r := gin.New()
	r.Use(RateLimit(limiter, cfg, log, nil))
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-RateLimit-Limit"))

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/login", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Empty(t, w2.Header().Get("X-RateLimit-Limit"))

	_ = client.Del(ctx, "ratelimit:ip:127.0.0.1")
}

func TestRateLimit_UnauthenticatedBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	cfg := &config.Config{
		RegisteredMessageLimit: 10,
		PremiumMessageLimit:    100,
	}
	log := logger.New("error")
	limiter := ratelimit.NewLimiter(client)

	r := gin.New()
	r.Use(RateLimit(limiter, cfg, log, nil))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}