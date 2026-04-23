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
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/runtime"
	"github.com/r0lm0/go-saas-api/internal/agent/store"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
	"github.com/r0lm0/go-saas-api/internal/platform/nats"
	"github.com/r0lm0/go-saas-api/internal/platform/postgres"
	"github.com/r0lm0/go-saas-api/internal/platform/redis"
	"github.com/r0lm0/go-saas-api/internal/provider"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// Server holds all HTTP handlers and dependencies.
type Server struct {
	orch            *runtime.Orchestrator
	convRepo        repository.ConversationRepo
	msgRepo         repository.MessageRepo
	artRepo         repository.ArtifactRepo
	fileHandler     *fileupload.Handler
	log             logger.Logger
	llmManager      *llm.MultiClient
	hc              *health.Checker
	providerHandler *provider.Handler
}

func newServer(orch *runtime.Orchestrator, convRepo repository.ConversationRepo, msgRepo repository.MessageRepo, artRepo repository.ArtifactRepo, fileHandler *fileupload.Handler, log logger.Logger, manager *llm.MultiClient, hc *health.Checker, providerHandler *provider.Handler) *Server {
	return &Server{orch: orch, convRepo: convRepo, msgRepo: msgRepo, artRepo: artRepo, fileHandler: fileHandler, log: log, llmManager: manager, hc: hc, providerHandler: providerHandler}
}

func (s *Server) setupRouter(jwtMgr *jwt.Manager) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(s.log))
	r.Use(middleware.CORS())

	r.GET("/health", s.handleHealthDetailed(s.hc))
	r.GET("/chat/models", s.handleListModels)
	r.POST("/agent/chat", s.handleAgentChat)
	r.POST("/artifacts", s.handleCreateArtifact)
	r.GET("/artifacts/:id/preview", s.handlePreviewArtifact)

	// Conversations REST API
	r.GET("/conversations", s.handleListConversations)
	r.GET("/conversations/:id", s.handleGetConversation)
	r.PATCH("/conversations/:id", s.handleUpdateConversation)
	r.DELETE("/conversations/:id", s.handleDeleteConversation)

	// Messages REST API
	r.GET("/conversations/:id/messages", s.handleListMessages)

	// File Upload API
	s.fileHandler.RegisterRoutes(r)

	// Admin provider routes (JWT required)
	if s.providerHandler != nil {
		s.providerHandler.RegisterRoutes(r, middleware.JWTAuth(jwtMgr))
	}

	return r
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "agent-service",
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) handleHealthDetailed(hc *health.Checker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if hc == nil {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"service":   "agent-service",
				"timestamp": time.Now().UTC(),
			})
			return
		}
		report := hc.Check(c.Request.Context())
		if !report.Healthy {
			c.JSON(http.StatusServiceUnavailable, report)
			return
		}
		c.JSON(http.StatusOK, report)
	}
}

func (s *Server) handleListModels(c *gin.Context) {
	providers := s.llmManager.ListProviders()
	models := make([]gin.H, 0)
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		for _, m := range p.Models {
			models = append(models, gin.H{
				"id":       m,
				"provider": p.Name,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (s *Server) handleAgentChat(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID or X-User-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	var req struct {
		ConversationID *uuid.UUID   `json:"conversation_id"`
		Message        string       `json:"message" binding:"required"`
		FileIDs        []uuid.UUID  `json:"file_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	streamCh, err := s.orch.Chat(ctx, tenantID, userID, req.ConversationID, req.Message, req.FileIDs)
	if err != nil {
		s.log.Error("chat failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "chat failed"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	for chunk := range streamCh {
		data := fmt.Sprintf("data: {\"content\":%q,\"done\":%v}\n\n", chunk.Content, chunk.Done)
		_, _ = c.Writer.Write([]byte(data))
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		if chunk.Done {
			break
		}
	}
}

func (s *Server) handleCreateArtifact(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	var req struct {
		ConversationID uuid.UUID `json:"conversation_id" binding:"required"`
		Name           string    `json:"name" binding:"required"`
		Type           string    `json:"type" binding:"required"`
		Language       string    `json:"language"`
		Content        string    `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	art := &model.Artifact{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ConversationID: req.ConversationID,
		Name:           req.Name,
		Type:           req.Type,
		Language:       req.Language,
		Content:        req.Content,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.artRepo.Create(c.Request.Context(), art); err != nil {
		s.log.Error("create artifact failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create artifact"})
		return
	}

	c.JSON(http.StatusCreated, art)
}

func (s *Server) handlePreviewArtifact(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	art, err := s.artRepo.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		s.log.Error("get artifact failed", logger.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, art.Content)
}

func (s *Server) handleListConversations(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID or X-User-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := c.Query("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}
	if limit > 100 {
		limit = 100
	}

	convs, err := s.convRepo.ListByUser(c.Request.Context(), tenantID, userID, limit, offset)
	if err != nil {
		s.log.Error("list conversations failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list conversations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversations": convs})
}

func (s *Server) handleGetConversation(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	conv, err := s.convRepo.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		s.log.Error("get conversation failed", logger.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	c.JSON(http.StatusOK, conv)
}

func (s *Server) handleUpdateConversation(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	conv, err := s.convRepo.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		s.log.Error("get conversation for update failed", logger.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	var req struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		conv.Title = req.Title
	}
	if req.Status != "" {
		conv.Status = model.ConversationStatus(req.Status)
	}
	conv.UpdatedAt = time.Now()

	if err := s.convRepo.Update(c.Request.Context(), conv); err != nil {
		s.log.Error("update conversation failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update conversation"})
		return
	}

	c.JSON(http.StatusOK, conv)
}

func (s *Server) handleDeleteConversation(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := s.convRepo.Delete(c.Request.Context(), tenantID, id); err != nil {
		s.log.Error("delete conversation failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted"})
}

func (s *Server) handleListMessages(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	msgs, err := s.msgRepo.ListByConversation(c.Request.Context(), tenantID, conversationID, limit)
	if err != nil {
		s.log.Error("list messages failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	defer log.Sync()

	log.Info("starting agent-service",
		logger.String("port", cfg.Port),
		logger.String("env", cfg.Env),
	)

	// Infrastructure connections
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

	nc, err := nats.NewConn(cfg.NATSURL)
	if err != nil {
		log.Fatal("failed to connect to nats", logger.Error(err))
	}
	defer nc.Close()

	// Agent runtime wiring
	multiClient := llm.NewMultiClient()

	// Provider management
	providerStore := provider.NewPostgresStore(pgPool)
	providerLoader := NewProviderLoader(providerStore, multiClient)

	ctx = context.Background()
	if err := providerLoader.LoadAll(ctx); err != nil {
		log.Warn("failed to load providers from database, using env fallback", logger.Error(err))
	}

	// Fallback: if no providers loaded from DB, register from env
	if len(multiClient.ListProviders()) == 0 {
		// Register Ollama (local, always available if URL set)
		multiClient.Register(llm.ProviderConfig{
			Name:    "ollama",
			Models:  []string{"qwen2.5-coder:7b", "deepseek-r1:7b", "llama3.2:3b"},
			Client:  llm.NewOllamaClient(cfg.OllamaURL),
			Enabled: cfg.OllamaURL != "",
			Weight:  50,
		})

		if cfg.OpenAIAPIKey != "" {
			multiClient.Register(llm.ProviderConfig{
				Name:    "openai",
				Models:  []string{"gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo"},
				Client:  llm.NewOpenAIClient(cfg.OpenAIAPIKey),
				Enabled: true,
				Weight:  100,
			})
		}

		if cfg.GeminiAPIKey != "" {
			multiClient.Register(llm.ProviderConfig{
				Name:    "gemini",
				Models:  []string{"gemini-1.5-flash", "gemini-1.5-pro"},
				Client:  llm.NewGeminiClient(cfg.GeminiAPIKey),
				Enabled: true,
				Weight:  80,
			})
		}

		if cfg.DeepSeekAPIKey != "" {
			multiClient.Register(llm.ProviderConfig{
				Name:    "deepseek",
				Models:  []string{"deepseek-chat", "deepseek-coder"},
				Client:  llm.NewDeepSeekClient(cfg.DeepSeekAPIKey),
				Enabled: true,
				Weight:  90,
			})
		}
	}

	llmClient := llm.Client(multiClient)

	// Provider admin service
	providerService := provider.NewService(providerStore)
	providerHandler := provider.NewHandler(providerService, log)

	convStore := store.NewConversationStore(pgPool)
	msgStore := store.NewMessageStore(pgPool)
	artStore := store.NewArtifactStore(pgPool)
	agentStore := store.NewAgentStore(pgPool)

	toolRegistry := tools.NewRegistry()
	_ = toolRegistry.Register(tools.NewFileWriteTool(artStore))
	_ = toolRegistry.Register(tools.NewCodeExecuteTool(tools.NewHTTPSandboxClient(cfg.SandboxServiceURL)))

	sessions := runtime.NewSessionManager(convStore, msgStore)

	fileStore := fileupload.NewPostgresStore(pgPool)
	fileService := fileupload.NewService(fileStore, cfg.UploadPath, cfg.MaxUploadSize)
	fileHandler := fileupload.NewHandler(fileService, log)

	orchestrator := runtime.NewOrchestrator(llmClient, toolRegistry, sessions, agentStore, fileService)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Health checker
	hc := health.NewChecker(pgPool, redisClient, nc)

	jwtMgr := jwt.NewManager(cfg.JWTSecret)
	srv := newServer(orchestrator, convStore, msgStore, artStore, fileHandler, log, multiClient, hc, providerHandler)
	r := srv.setupRouter(jwtMgr)
	r.Use(middleware.RequestTimeout(60 * time.Second))

	httpSrv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", logger.Error(err))
		}
	}()

	log.Info("agent-service running", logger.String("addr", httpSrv.Addr))

	<-quit
	log.Info("shutting down agent-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", logger.Error(err))
	}

	log.Info("agent-service exited")
}
