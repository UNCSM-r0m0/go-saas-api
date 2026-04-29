package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit_Anonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	cfg := &config.Config{
		FreeMessageLimit:       2,
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

	// 1st request: allowed
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "2", w1.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "1", w1.Header().Get("X-RateLimit-Remaining"))

	// 2nd request: allowed
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "0", w2.Header().Get("X-RateLimit-Remaining"))

	// 3rd request: denied (429)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.1:1234"
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)

	// Cleanup
	_ = client.Del(ctx, "ratelimit:ip:192.168.1.1")
}

func TestRateLimit_AnonymousIdFromBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	cfg := &config.Config{
		FreeMessageLimit:       2,
		RegisteredMessageLimit: 10,
		PremiumMessageLimit:    100,
	}
	log := logger.New("error")
	limiter := ratelimit.NewLimiter(client)

	r := gin.New()
	r.Use(RateLimit(limiter, cfg, log, nil))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Request with anonymousId in body: should use anon key, not IP
	body := `{"anonymousId":"device-abc-123","message":"hello"}`
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/test", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.RemoteAddr = "192.168.1.1:1234"
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "2", w1.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "1", w1.Header().Get("X-RateLimit-Remaining"))

	// Second request with same anonymousId from different IP: should share bucket
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/test", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.RemoteAddr = "192.168.1.2:5678"
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "0", w2.Header().Get("X-RateLimit-Remaining"))

	// Third request with same anonymousId: should be blocked (429)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/test", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.RemoteAddr = "192.168.1.3:9999"
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)

	// Cleanup
	_ = client.Del(ctx, "ratelimit:anon:device-abc-123")
}

func TestRateLimit_SkipsAuthAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available:", err)
	}

	cfg := &config.Config{FreeMessageLimit: 1}
	log := logger.New("error")
	limiter := ratelimit.NewLimiter(client)

	r := gin.New()
	r.Use(RateLimit(limiter, cfg, log, nil))
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	// Health check should skip rate limit
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-RateLimit-Limit"))

	// Auth endpoint should skip rate limit
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/login", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Empty(t, w2.Header().Get("X-RateLimit-Limit"))

	// Cleanup
	_ = client.Del(ctx, "ratelimit:ip:127.0.0.1")
}
