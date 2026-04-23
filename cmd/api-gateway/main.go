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
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
)

func main() {
	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Inicializar logger
	log := logger.New(cfg.LogLevel)
	defer log.Sync()

	log.Info("starting api-gateway",
		logger.String("port", cfg.Port),
		logger.String("env", cfg.Env),
	)

	// Configurar Gin
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
	r.Use(middleware.CORS())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "api-gateway",
			"timestamp": time.Now().UTC(),
		})
	})

	// API v1 routes — proxy a servicios
	v1 := r.Group("/api/v1")
	{
		// Agent service proxy
		v1.Any("/chat/*path", proxyToService(cfg.AgentServiceURL, log))
		
		// Auth service proxy
		v1.Any("/auth/*path", proxyToService(cfg.AuthServiceURL, log))
		
		// Billing service proxy
		v1.Any("/billing/*path", proxyToService(cfg.BillingServiceURL, log))
		
		// Usage service proxy
		v1.Any("/usage/*path", proxyToService(cfg.UsageServiceURL, log))
	}

	// Crear servidor HTTP
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown
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

// proxyToService crea un reverse proxy hacia un microservicio
func proxyToService(targetURL string, log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implementar reverse proxy real con http.ReverseProxy
		// Por ahora, forward básico
		c.JSON(http.StatusOK, gin.H{
			"message":    "proxy to " + targetURL + c.Param("path"),
			"request_id": c.GetString("request_id"),
		})
	}
}
