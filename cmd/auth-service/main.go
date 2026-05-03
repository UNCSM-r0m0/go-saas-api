package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/apikey"
	"github.com/r0lm0/go-saas-api/internal/auth"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
	"github.com/r0lm0/go-saas-api/internal/platform/postgres"
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

	log.Info("starting auth-service",
		logger.String("port", cfg.Port),
		logger.String("env", cfg.Env),
	)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Infrastructure
	ctx := context.Background()
	pgPool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to postgres", logger.Error(err))
	}
	defer pgPool.Close()

	redisClient, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to connect to redis", logger.Error(err))
	}
	defer redisClient.Close()

	// Auth layer
	jwtMgr := jwt.NewManager(cfg.JWTSecret)
	userStore := auth.NewPostgresUserStore(pgPool)
	prefsStore := auth.NewPostgresPreferencesStore(pgPool)
	refreshStore := auth.NewRedisRefreshTokenStore(redisClient)
	resetStore := auth.NewRedisPasswordResetTokenStore(redisClient)

	var emailSender auth.EmailSender
	if cfg.SMTPHost != "" {
		emailSender = auth.NewSMTPEmailSender(auth.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		})
	} else {
		emailSender = auth.NewNoopEmailSender()
	}

	authService := auth.NewService(userStore, refreshStore, resetStore, emailSender, cfg.FrontendURL, jwtMgr, cfg.JWTExpiration, 7*24*time.Hour, 15*time.Minute)
	authHandler := auth.NewHandler(authService, log)
	prefsHandler := auth.NewPreferencesHandler(prefsStore, log)

	// OAuth layer
	oauthStateStore := auth.NewRedisOAuthStateStore(redisClient)
	oauthService := auth.NewOAuthService(
		userStore, refreshStore, jwtMgr, oauthStateStore,
		cfg.GoogleClientID, cfg.GoogleClientSecret, fmt.Sprintf("%s/api/v1/auth/google/callback", cfg.PublicURL),
		cfg.GitHubClientID, cfg.GitHubClientSecret, fmt.Sprintf("%s/api/v1/auth/github/callback", cfg.PublicURL),
		cfg.JWTExpiration, 7*24*time.Hour,
	)
	oauthHandler := auth.NewOAuthHandler(oauthService, log)

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

	// Auth endpoints
	authHandler.RegisterRoutes(r)
	oauthHandler.RegisterRoutes(r)

	// Admin routes
	adminHandler := auth.NewAdminHandler(userStore, log)
	adminHandler.RegisterRoutes(r, middleware.JWTAuth(jwtMgr), middleware.AdminOnly())

	// API key management
	apiKeyStore := apikey.NewPostgresStore(pgPool)
	apiKeyService := apikey.NewService(apiKeyStore)
	apiKeyHandler := apikey.NewHandler(apiKeyService, log)
	apiKeyHandler.RegisterRoutes(r, middleware.JWTAuth(jwtMgr))

	// Protected auth endpoints (validates its own JWT if called directly)
	protectedAuth := r.Group("/auth")
	protectedAuth.Use(middleware.JWTAuth(jwtMgr))
	protectedAuth.GET("/me", authHandler.Me)
	protectedAuth.GET("/profile", authHandler.Me) // alias for r3-chat frontend

	// User profile routes (protected)
	users := r.Group("/users")
	users.Use(middleware.JWTAuth(jwtMgr))
	users.PUT("/profile", authHandler.UpdateProfile)
	prefsHandler.RegisterRoutes(r, middleware.JWTAuth(jwtMgr))

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

	log.Info("auth-service running", logger.String("addr", srv.Addr))

	<-quit
	log.Info("shutting down auth-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", logger.Error(err))
	}

	log.Info("auth-service exited")
}
