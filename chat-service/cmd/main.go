package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ChatRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream,omitempty"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var (
	logger       *zap.Logger
	ollamaURL    string
	useProxy     bool
	proxyURL     string
	proxyAPIKey  string
	defaultModel string
)

func main() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	port := getEnv("PORT", "3002")
	ollamaURL = getEnv("OLLAMA_URL", "http://localhost:11434")
	proxyURL = getEnv("OLLAMA_PROXY_URL", "")
	proxyAPIKey = getEnv("OLLAMA_PROXY_API_KEY", "")
	useProxy = proxyURL != ""

	if useProxy {
		defaultModel = getFirstModel(getEnv("PUBLIC_MODELS", "qwen2.5-coder:7b"))
	} else {
		defaultModel = getEnv("OLLAMA_MODEL", "qwen2.5-coder:7b")
	}

	logger.Info("Chat Service starting",
		zap.String("port", port),
		zap.String("ollamaURL", ollamaURL),
		zap.Bool("useProxy", useProxy),
	)

	r := gin.Default()
	r.GET("/health", healthHandler)
	r.GET("/chat/models", listModelsHandler)
	r.POST("/chat/completions", chatHandler)
	r.POST("/chat/stream", streamChatHandler)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	logger.Info("Chat Service started", zap.String("port", port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("Server exited")
}

func healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "ok",
		"service":   "chat-service",
		"timestamp": time.Now().Unix(),
	})
}

func listModelsHandler(c *gin.Context) {
	// Simplified - returns default models
	models := []map[string]string{
		{"id": defaultModel, "name": defaultModel},
	}
	c.JSON(200, gin.H{"models": models})
}

func chatHandler(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Model == "" {
		req.Model = defaultModel
	}

	c.JSON(200, gin.H{
		"message": "Chat endpoint - implement Ollama integration",
		"model":   req.Model,
	})
}

func streamChatHandler(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Model == "" {
		req.Model = defaultModel
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Mock streaming for now
	messageID := uuid.New().String()
	c.Stream(func(w io.Writer) bool {
		c.SSEvent("message", `{"id":"`+messageID+`","choices":[{"delta":{"content":"Hello from Go!"}}]}`)
		time.Sleep(100 * time.Millisecond)
		c.SSEvent("message", `{"id":"`+messageID+`","choices":[{"delta":{"content":" Streaming works!"}}]}`)
		return false
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getFirstModel(models string) string {
	if models == "" {
		return "qwen2.5-coder:7b"
	}
	parts := strings.Split(models, ",")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return models
}
