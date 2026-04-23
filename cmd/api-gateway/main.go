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
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
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

	jwtMgr := jwt.NewManager(cfg.JWTSecret)

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

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Public auth routes (no JWT required)
		v1.POST("/auth/register", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/login", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/refresh", proxyTo(cfg.AuthServiceURL, "/api/v1"))
		v1.POST("/auth/logout", proxyTo(cfg.AuthServiceURL, "/api/v1"))

		// Protected auth route (gateway validates JWT)
		v1.GET("/auth/me", middleware.JWTAuth(jwtMgr), proxyTo(cfg.AuthServiceURL, "/api/v1"))

		// Protected service routes
		protected := v1.Group("")
		protected.Use(middleware.JWTAuth(jwtMgr))
		{
			protected.Any("/agent/*path", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			protected.Any("/artifacts", proxyTo(cfg.AgentServiceURL, "/api/v1"))
			protected.Any("/artifacts/*path", proxyTo(cfg.AgentServiceURL, "/api/v1"))
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
