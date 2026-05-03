package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/r0lm0/go-saas-api/docs"
	"github.com/r0lm0/go-saas-api/internal/apikey"
	"github.com/r0lm0/go-saas-api/internal/billing"
	"github.com/r0lm0/go-saas-api/internal/platform/cache"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
	"github.com/r0lm0/go-saas-api/internal/platform/nats"
	"github.com/r0lm0/go-saas-api/internal/platform/postgres"
	"github.com/r0lm0/go-saas-api/internal/platform/ratelimit"
	"github.com/r0lm0/go-saas-api/internal/platform/redis"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
	natsio "github.com/nats-io/nats.go"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	defer log.Sync()

	log.Info("starting api-gateway",
		logger.String("port", cfg.Port),
		logger.String("env", cfg.Env),
	)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Infrastructure
	ctx := context.Background()

	redisClient, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to connect to redis", logger.Error(err))
	}
	defer redisClient.Close()

	pgPool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to postgres", logger.Error(err))
	}
	defer pgPool.Close()

	jwtMgr := jwt.NewManager(cfg.JWTSecret)
	rateLimiter := ratelimit.NewLimiter(redisClient)

	// Tier resolver for premium rate limits (with Redis cache)
	billingStore := billing.NewPostgresBillingStore(pgPool)
	appCache := cache.NewCache(redisClient, "saas")
	baseResolver := &dbTierResolver{store: billingStore}
	tierResolver := &cachedTierResolver{
		cache:    appCache,
		fallback: baseResolver,
		ttl:      5 * time.Minute,
	}

	// Health checker
	hc := health.NewChecker(pgPool, redisClient, nil)

	// API key service for gateway-level authentication
	apiKeyStore := apikey.NewPostgresStore(pgPool)
	apiKeyService := apikey.NewService(apiKeyStore)

	// NATS consumer for cache invalidation
	if cfg.NATSEventsEnabled {
		nc, err := nats.NewConn(cfg.NATSURL)
		if err != nil {
			log.Warn("failed to connect to nats, continuing without events", logger.Error(err))
		} else {
			defer nc.Close()
			// Subscribe to usage.recorded and subscription.changed
			_, _ = nc.Subscribe("usage.recorded", func(msg *natsio.Msg) {
				var event struct {
					UserID string `json:"user_id"`
				}
				if err := json.Unmarshal(msg.Data, &event); err == nil {
					key := fmt.Sprintf("tier:%s", event.UserID)
					_ = redisClient.Del(context.Background(), key).Err()
					log.Info("nats: invalidated cache", logger.String("key", key))
				}
			})
			_, _ = nc.Subscribe("subscription.changed", func(msg *natsio.Msg) {
				var event struct {
					UserID string `json:"user_id"`
				}
				if err := json.Unmarshal(msg.Data, &event); err == nil {
					key := fmt.Sprintf("tier:%s", event.UserID)
					_ = redisClient.Del(context.Background(), key).Err()
					log.Info("nats: invalidated cache", logger.String("key", key))
				}
			})
			log.Info("nats cache invalidation consumer started")
		}
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
	r.Use(middleware.CORS())
	r.Use(middleware.RequestTimeout(30 * time.Second))

	// Swagger UI (development only)
	if cfg.Env != "production" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		report := hc.Check(c.Request.Context())
		if !report.Healthy {
			c.JSON(http.StatusServiceUnavailable, report)
			return
		}
		c.JSON(http.StatusOK, report)
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Public auth routes (no JWT, no rate limit)
		v1.POST("/auth/register", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/login", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/refresh", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/logout", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/forgot-password", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/reset-password", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/google", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/google/callback", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/github", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/github/callback", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/callback", proxyTo(cfg.AuthServiceURL, "/api/v1"))

		// Protected auth routes (gateway validates JWT or API key)
		v1.GET("/auth/me", middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService), proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/profile", middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService), proxyTo(cfg.AuthServiceURL, "/api/v1"))

		// Protected user routes
		v1.PUT("/users/profile", middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService), proxyTo(cfg.AuthServiceURL, "/api/v1"))

		// Public agent routes (no auth required)
		v1.GET("/chat/models", proxyTo(cfg.AgentServiceURL, "/api/v1"))
		v1.GET("/models/public", proxyTo(cfg.AgentServiceURL, "/api/v1"))

		// Agent routes: require auth (JWT or API key) + rate limit
		agent := v1.Group("")
		agent.Use(middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService))
		agent.Use(middleware.RateLimit(rateLimiter, cfg, log, tierResolver))
		{
			// WebSocket upgrade at v1 level (avoids Gin wildcard conflict with /agent/*path)
			v1.GET("/agent/ws",
				middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService),
				middleware.RateLimit(rateLimiter, cfg, log, tierResolver),
				proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.POST("/agent/chat", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.Any("/artifacts", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.Any("/artifacts/*path", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			// r3-chat frontend chat routes (explicit to avoid conflict with /chat/models)
			agent.POST("/chat", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.GET("/chat/sessions", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.GET("/chat/:id", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.PATCH("/chat/sessions/:id", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.DELETE("/chat/sessions/:id", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.POST("/chat/message", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.POST("/chat/message/stream", proxyTo(cfg.AgentServiceURL, "/api/v1"))
		}

		// API key management routes (JWT required)
		apiKeys := v1.Group("")
		apiKeys.Use(middleware.JWTAuth(jwtMgr))
		apiKeys.Use(middleware.RateLimit(rateLimiter, cfg, log, tierResolver))
		{
			apiKeys.Any("/api-keys", proxyTo(cfg.AuthServiceURL, "/api/v1"))
			apiKeys.Any("/api-keys/*path", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		}

		// Sandbox routes (JWT or API key + strict sandbox rate limit + 15s timeout)
		sandbox := v1.Group("/sandbox")
		sandbox.Use(middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService))
		sandbox.Use(middleware.SandboxRateLimit(rateLimiter, cfg, log))
		sandbox.Use(middleware.RequestTimeout(15 * time.Second))
		{
			sandbox.Any("/*path", proxyTo(cfg.SandboxServiceURL, "/api/v1/sandbox"))
		}

		// Document routes (JWT or API key + rate limit)
		documents := v1.Group("")
		documents.Use(middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService))
		documents.Use(middleware.RateLimit(rateLimiter, cfg, log, tierResolver))
		{
			documents.POST("/files/upload", uploadSizeCheck(cfg.MaxUploadSize), proxyTo(cfg.DocumentServiceURL, "/api/v1"))
			documents.GET("/documents/:id", proxyTo(cfg.DocumentServiceURL, "/api/v1"))
		}

		// Usage stats route (JWT or API key + rate limit)
		v1.GET("/chat/usage/stats",
			middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService),
			middleware.RateLimit(rateLimiter, cfg, log, tierResolver),
			proxyToWithPrefix(cfg.UsageServiceURL, "/api/v1", "/usage"))

		// Protected service routes (JWT or API key required + rate limit)
		protected := v1.Group("")
		protected.Use(middleware.JWTOrAPIKeyAuth(jwtMgr, apiKeyService))
		protected.Use(middleware.RateLimit(rateLimiter, cfg, log, tierResolver))
		{
			protected.Any("/conversations", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			protected.Any("/conversations/*path", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			protected.Any("/billing/*path", proxyTo(cfg.BillingServiceURL, "/api/v1"))
			// r3-chat frontend billing routes — billing service registers these under /billing
			protected.Any("/stripe/*path", proxyToWithPrefix(cfg.BillingServiceURL, "/api/v1", "/billing"))
			protected.Any("/subscriptions", proxyToWithPrefix(cfg.BillingServiceURL, "/api/v1", "/billing"))
			protected.Any("/subscriptions/*path", proxyToWithPrefix(cfg.BillingServiceURL, "/api/v1", "/billing"))
			protected.Any("/usage/*path", proxyTo(cfg.UsageServiceURL, "/api/v1"))
		}
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", logger.Error(err))
		}
	}()

	log.Info("api-gateway running", logger.String("addr", srv.Addr))

	<- quit
	log.Info("shutting down api-gateway")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", logger.Error(err))
	}

	log.Info("api-gateway exited")
}

// proxyTo creates a reverse proxy to a target service, optionally stripping a path prefix
func proxyTo(targetURL, stripPrefix string) gin.HandlerFunc {
	return proxyToWithPrefix(targetURL, stripPrefix, "")
}

// proxyToWithPrefix creates a reverse proxy that strips a prefix and adds another prefix.
func proxyToWithPrefix(targetURL, stripPrefix, addPrefix string) gin.HandlerFunc {
	target, err := url.Parse(targetURL)
	if err != nil {
		panic(fmt.Sprintf("invalid target URL %s: %v", targetURL, err))
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1 // stream SSE/WebSocket chunks immediately
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if stripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		if addPrefix != "" {
			req.URL.Path = addPrefix + req.URL.Path
		}
	}

	// Inject auth context from Gin into outgoing request headers
	// so downstream services receive X-User-ID even
	// when the client only sent an Authorization header.
	// For unauthenticated requests, inject X-Anonymous-ID (anonymousId from body or ClientIP).
	injectAuth := func(c *gin.Context) {
		if uid, ok := c.Get("user_id"); ok && uid != "" {
			c.Request.Header.Set("X-User-ID", fmt.Sprintf("%v", uid))
		}
		if role, ok := c.Get("role"); ok && role != "" {
			c.Request.Header.Set("X-User-Role", fmt.Sprintf("%v", role))
		}
		// Anonymous identification for unauthenticated requests
		if _, ok := c.Get("user_id"); !ok {
			anonID := extractAnonymousID(c)
			if anonID != "" {
				c.Request.Header.Set("X-Anonymous-ID", anonID)
			}
		}
	}

	// Remove CORS headers from backend response so the gateway's CORS
	// middleware is the single source of truth and avoids duplicates.
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Access-Control-Allow-Origin")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Allow-Credentials")
		resp.Header.Del("Access-Control-Expose-Headers")
		resp.Header.Del("Access-Control-Max-Age")
		return nil
	}

	// Error handler for proxy failures
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":"service unavailable","details":"%s"}`, err.Error())
	}

	return func(c *gin.Context) {
		injectAuth(c)
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// extractAnonymousID extracts an anonymous identifier from the request body or falls back to ClientIP.
func extractAnonymousID(c *gin.Context) string {
	// Try to read anonymousId from JSON body (best effort)
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
			// Restore body even if parsing failed
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
	}
	return c.ClientIP()
}

// uploadSizeCheck returns a middleware that validates Content-Length against max size.
// It returns 413 before proxying if the upload exceeds the limit.
func uploadSizeCheck(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":       "upload too large",
				"max_size_mb": maxSize / (1024 * 1024),
			})
			return
		}
		c.Next()
	}
}

// dbTierResolver resolves user tiers from the billing database.
type dbTierResolver struct {
	store billing.SubscriptionRepository
}

func (r *dbTierResolver) ResolveTier(ctx context.Context, userID string) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}

	sub, err := r.store.GetSubscriptionByUser(ctx, uid)
	if err != nil {
		return "", err
	}
	if sub == nil || sub.Status != "active" {
		return "registered", nil
	}

	// Map plan IDs to tiers. For simplicity we look at the plan slug via a separate query,
	// but here we infer from amount or just return premium for any active paid sub.
	// A more robust solution would cache plan slugs.
	// For now: if there's an active subscription with a stripe sub ID, treat as premium.
	if sub.StripeSubscriptionID != "" {
		return "premium", nil
	}
	return "registered", nil
}

// cachedTierResolver wraps a TierResolver with Redis caching.
type cachedTierResolver struct {
	cache    *cache.Cache
	fallback middleware.TierResolver
	ttl      time.Duration
}

func (c *cachedTierResolver) ResolveTier(ctx context.Context, userID string) (string, error) {
	cacheKey := fmt.Sprintf("tier:%s", userID)
	var tier string
	if err := c.cache.Get(ctx, cacheKey, &tier); err == nil {
		return tier, nil
	}

	tier, err := c.fallback.ResolveTier(ctx, userID)
	if err != nil {
		return "", err
	}

	_ = c.cache.Set(ctx, cacheKey, tier, c.ttl)
	return tier, nil
}
