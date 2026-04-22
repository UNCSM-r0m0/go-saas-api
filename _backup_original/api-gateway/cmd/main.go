package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Config
	port := getEnv("PORT", "3001")
	
	// Router
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.Default())
	
	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"service": "api-gateway",
			"timestamp": time.Now().Unix(),
		})
	})

	// Routes
	setupRoutes(r, logger)

	// Server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	logger.Info("API Gateway started", zap.String("port", port))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func setupRoutes(r *gin.Engine, logger *zap.Logger) {
	// API v1
	v1 := r.Group("/api/v1")
	{
		// Auth routes (proxy to auth-service)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", proxyToService("AUTH_SERVICE_URL", "/auth/register", logger))
			auth.POST("/login", proxyToService("AUTH_SERVICE_URL", "/auth/login", logger))
			auth.POST("/refresh", proxyToService("AUTH_SERVICE_URL", "/auth/refresh", logger))
			auth.GET("/google", proxyToService("AUTH_SERVICE_URL", "/auth/google", logger))
			auth.GET("/google/callback", proxyToService("AUTH_SERVICE_URL", "/auth/google/callback", logger))
		}

		// Chat routes (proxy to chat-service)
		chat := v1.Group("/chat")
		{
			chat.POST("/completions", proxyToService("CHAT_SERVICE_URL", "/chat/completions", logger))
			chat.POST("/stream", proxyToService("CHAT_SERVICE_URL", "/chat/stream", logger))
			chat.GET("/models", proxyToService("CHAT_SERVICE_URL", "/chat/models", logger))
			chat.GET("/history", proxyToService("CHAT_SERVICE_URL", "/chat/history", logger))
		}

		// Billing routes
		billing := v1.Group("/billing")
		{
			billing.GET("/plans", proxyToService("BILLING_SERVICE_URL", "/billing/plans", logger))
			billing.POST("/subscribe", proxyToService("BILLING_SERVICE_URL", "/billing/subscribe", logger))
			billing.POST("/webhook", proxyToService("BILLING_SERVICE_URL", "/billing/webhook", logger))
		}

		// Usage routes
		usage := v1.Group("/usage")
		{
			usage.GET("/stats", proxyToService("USAGE_SERVICE_URL", "/usage/stats", logger))
			usage.GET("/limits", proxyToService("USAGE_SERVICE_URL", "/usage/limits", logger))
		}
	}
}

func proxyToService(envVar, path string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceURL := getEnv(envVar, "")
		if serviceURL == "" {
			c.JSON(503, gin.H{"error": "Service unavailable"})
			return
		}

		// TODO: Implement proper reverse proxy with load balancing
		c.JSON(200, gin.H{
			"message": "Proxy to " + serviceURL + path,
			"status": "not fully implemented yet",
		})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
