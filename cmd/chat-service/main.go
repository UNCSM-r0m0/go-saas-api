package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/shared/config"
	"github.com/r0lm0/go-saas-api/internal/shared/logger"
	"github.com/r0lm0/go-saas-api/internal/shared/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	defer log.Sync()

	log.Info("starting chat-service",
		logger.String("port", cfg.Port),
		logger.String("env", cfg.Env),
	)

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
			"service":   "chat-service",
			"timestamp": time.Now().UTC(),
		})
	})

	// Chat endpoints
	r.GET("/chat/models", handleListModels)
	r.POST("/chat/completions", handleCompletion)
	r.POST("/chat/stream", handleStream)
	r.GET("/chat/history/:conversationId", handleGetHistory)

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

	log.Info("chat-service running", logger.String("addr", srv.Addr))

	<-quit
	log.Info("shutting down chat-service")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", logger.Error(err))
	}

	log.Info("chat-service exited")
}

func handleListModels(c *gin.Context) {
	// TODO: Leer modelos desde BD
	c.JSON(http.StatusOK, gin.H{
		"models": []gin.H{
			{"id": "qwen2.5-coder:7b", "name": "Qwen 2.5 Coder 7B", "provider": "ollama"},
			{"id": "deepseek-r1:7b", "name": "DeepSeek R1 7B", "provider": "ollama"},
		},
	})
}

func handleCompletion(c *gin.Context) {
	// TODO: Implementar chat completion síncrono
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

func handleStream(c *gin.Context) {
	// TODO: Implementar SSE streaming real con provider
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"chunk":"streaming not yet implemented"}`)
		return false
	})
}

func handleGetHistory(c *gin.Context) {
	// TODO: Leer historial de conversación desde BD
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
