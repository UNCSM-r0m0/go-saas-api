package main

import (
	"context"
	"fmt"
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
	"github.com/r0lm0/go-saas-api/internal/billing"
	"github.com/r0lm0/go-saas-api/internal/platform/cache"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
	"github.com/r0lm0/go-saas-api/internal/platform/postgres"
	"github.com/r0lm0/go-saas-api/internal/platform/ratelimit"
	"github.com/r0lm0/go-saas-api/internal/platform/redis"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
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

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
	r.Use(middleware.CORS())
	r.Use(middleware.RequestTimeout(30 * time.Second))

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
		v1.GET("/auth/google", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/google/callback", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/github", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.GET("/auth/github/callback", proxyTo(cfg.AuthServiceURL, "/api/v1"))

		// Protected auth route (gateway validates JWT)
		v1.GET("/auth/me", middleware.JWTAuth(jwtMgr), proxyTo(cfg.AuthServiceURL, "/api/v1"))

		// Agent routes: optional auth + rate limit
		// Anonymous users get free tier (IP-based); authenticated get tier from DB
		agent := v1.Group("")
		agent.Use(middleware.JWTAuthOptional(jwtMgr))
		agent.Use(middleware.RateLimit(rateLimiter, cfg, log, tierResolver))
		{
			agent.Any("/agent/*path", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.Any("/artifacts", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			agent.Any("/artifacts/*path", proxyTo(cfg.AgentServiceURL, "/api/v1"))
		}

		// Protected service routes (JWT required + rate limit)
		protected := v1.Group("")
		protected.Use(middleware.JWTAuth(jwtMgr))
		protected.Use(middleware.RateLimit(rateLimiter, cfg, log, tierResolver))
		{
			protected.Any("/billing/*path", proxyTo(cfg.BillingServiceURL, "/api/v1"))
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

	<-quit
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
	target, err := url.Parse(targetURL)
	if err != nil {
		panic(fmt.Sprintf("invalid target URL %s: %v", targetURL, err))
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if stripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
	}

	// Error handler for proxy failures
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":"service unavailable","details":"%s"}`, err.Error())
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// dbTierResolver resolves user tiers from the billing database.
type dbTierResolver struct {
	store billing.SubscriptionRepository
}

func (r *dbTierResolver) ResolveTier(ctx context.Context, tenantID, userID string) (string, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return "", err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}

	sub, err := r.store.GetSubscriptionByUser(ctx, tid, uid)
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

func (c *cachedTierResolver) ResolveTier(ctx context.Context, tenantID, userID string) (string, error) {
	cacheKey := fmt.Sprintf("tier:%s:%s", tenantID, userID)
	var tier string
	if err := c.cache.Get(ctx, cacheKey, &tier); err == nil {
		return tier, nil
	}

	tier, err := c.fallback.ResolveTier(ctx, tenantID, userID)
	if err != nil {
		return "", err
	}

	_ = c.cache.Set(ctx, cacheKey, tier, c.ttl)
	return tier, nil
}
