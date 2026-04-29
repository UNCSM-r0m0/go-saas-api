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
	"github.com/r0lm0/go-saas-api/internal/billing"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
	"github.com/r0lm0/go-saas-api/internal/platform/nats"
	"github.com/r0lm0/go-saas-api/internal/platform/postgres"
	natsio "github.com/nats-io/nats.go"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	defer log.Sync()

	log.Info("starting billing-service",
		logger.String("port", cfg.Port),
		logger.String("env", cfg.Env),
	)

	ctx := context.Background()
	pgPool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to postgres", logger.Error(err))
	}
	defer pgPool.Close()

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// NATS connection (optional)
	var nc *natsio.Conn
	if cfg.NATSEventsEnabled {
		var err error
		nc, err = nats.NewConn(cfg.NATSURL)
		if err != nil {
			log.Warn("failed to connect to nats, continuing without events", logger.Error(err))
		} else {
			defer nc.Close()
		}
	}

	// Billing wiring
	store := billing.NewPostgresBillingStore(pgPool)
	billingService := billing.NewBillingService(store, store, cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.FrontendURL, nc)
	billingHandler := billing.NewHandler(billingService)

	// Health checker
	hc := health.NewChecker(pgPool, nil, nc)

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

	// Billing routes
	billingHandler.RegisterRoutes(r.Group("/billing"))

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

	log.Info("billing-service running", logger.String("addr", srv.Addr))

	<-quit
	log.Info("shutting down billing-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", logger.Error(err))
	}

	log.Info("billing-service exited")
}
