package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/runtime"
	"github.com/r0lm0/go-saas-api/internal/agent/store"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/agent/websocket"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
	"github.com/r0lm0/go-saas-api/internal/platform/nats"
	"github.com/r0lm0/go-saas-api/internal/platform/postgres"
	"github.com/r0lm0/go-saas-api/internal/platform/redis"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
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
	wsManager       *websocket.Manager
}

func newServer(orch *runtime.Orchestrator, convRepo repository.ConversationRepo, msgRepo repository.MessageRepo, artRepo repository.ArtifactRepo, fileHandler *fileupload.Handler, log logger.Logger, manager *llm.MultiClient, hc *health.Checker, providerHandler *provider.Handler, wsManager *websocket.Manager) *Server {
	return &Server{orch: orch, convRepo: convRepo, msgRepo: msgRepo, artRepo: artRepo, fileHandler: fileHandler, log: log, llmManager: manager, hc: hc, providerHandler: providerHandler, wsManager: wsManager}
}

func (s *Server) setupRouter(jwtMgr *jwt.Manager) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(s.log))
	r.Use(middleware.CORS())

	r.GET("/health", s.handleHealthDetailed(s.hc))
	r.GET("/chat/models", s.handleListModels)
	r.GET("/models/public", s.handleListModelsPublic)
	r.POST("/agent/chat", s.handleAgentChat)
	r.GET("/agent/ws", s.wsManager.HandleUpgrade)
	r.POST("/artifacts", s.handleCreateArtifact)
	r.GET("/artifacts/:id/preview", s.handlePreviewArtifact)

	// Conversations REST API (legacy)
	r.GET("/conversations", s.handleListConversations)
	r.GET("/conversations/:id", s.handleGetConversation)
	r.PATCH("/conversations/:id", s.handleUpdateConversation)
	r.DELETE("/conversations/:id", s.handleDeleteConversation)
	r.GET("/conversations/:id/messages", s.handleListMessages)

	// r3-chat frontend compatible chat routes
	r.GET("/chat/sessions", s.handleListConversations)
	r.POST("/chat", s.handleCreateChat)
	r.GET("/chat/:id", s.handleGetChatWithMessages)
	r.PATCH("/chat/sessions/:id", s.handleUpdateConversation)
	r.DELETE("/chat/sessions/:id", s.handleDeleteConversation)
	r.POST("/chat/message", s.handleChatMessage)
	r.POST("/chat/message/stream", s.handleChatMessageStream)

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

func (s *Server) handleListModelsPublic(c *gin.Context) {
	providers := s.llmManager.ListProviders()
	models := make([]gin.H, 0)
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		for _, m := range p.Models {
			models = append(models, gin.H{
				"id":                m,
				"name":              m,
				"provider":          p.Name,
				"description":       "Model " + m + " via " + p.Name,
				"maxTokens":         4096,
				"supportsImages":    false,
				"supportsReasoning": false,
				"isPremium":         p.Name != "ollama",
				"isAvailable":       true,
				"available":         true,
				"features":          []string{},
			})
		}
	}
	response.OK(c, models, "models retrieved")
}

func (s *Server) handleCreateChat(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing X-Tenant-ID or X-User-ID")
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid tenant_id")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	var req struct {
		Title string `json:"title" binding:"required"`
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	conv := &model.Conversation{
		ID:        uuid.New(),
		TenantID:  tenantID,
		UserID:    userID,
		Title:     req.Title,
		Status:    model.ConversationActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if req.Model != "" {
		conv.Metadata = map[string]any{"model": req.Model}
	}

	if err := s.convRepo.Create(c.Request.Context(), conv); err != nil {
		s.log.Error("create chat failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to create chat")
		return
	}

	response.Created(c, conv, "chat created")
}

func (s *Server) handleGetChatWithMessages(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing X-Tenant-ID")
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid tenant_id")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	conv, err := s.convRepo.GetByID(c.Request.Context(), tenantID, id)
	if err != nil {
		s.log.Error("get chat failed", logger.Error(err))
		response.Error(c, http.StatusNotFound, "chat not found")
		return
	}

	msgs, err := s.msgRepo.ListByConversation(c.Request.Context(), tenantID, id, 100)
	if err != nil {
		s.log.Error("list messages failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to load messages")
		return
	}

	conv.Messages = msgs
	response.OK(c, conv, "chat retrieved")
}

func (s *Server) handleChatMessage(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing X-Tenant-ID or X-User-ID")
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid tenant_id")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	var req struct {
		Content        string     `json:"content" binding:"required"`
		Model          string     `json:"model"`
		Context        string     `json:"context"`
		ConversationID *uuid.UUID `json:"conversationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	streamCh, err := s.orch.Chat(ctx, tenantID, userID, req.ConversationID, req.Content, nil)
	if err != nil {
		s.log.Error("chat failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "chat failed")
		return
	}

	var fullContent strings.Builder
	for chunk := range streamCh {
		fullContent.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}

	// The orchestrator already saved the assistant message.
	// Fetch the conversation to return it.
	var convID uuid.UUID
	if req.ConversationID != nil {
		convID = *req.ConversationID
	} else {
		// Find the most recent conversation for this user
		convs, err := s.convRepo.ListByUser(ctx, tenantID, userID, 1, 0)
		if err != nil || len(convs) == 0 {
			response.Error(c, http.StatusInternalServerError, "failed to retrieve conversation")
			return
		}
		convID = convs[0].ID
	}

	conv, err := s.convRepo.GetByID(ctx, tenantID, convID)
	if err != nil {
		s.log.Error("get conversation after chat failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to retrieve conversation")
		return
	}

	msgs, err := s.msgRepo.ListByConversation(ctx, tenantID, convID, 100)
	if err != nil {
		s.log.Error("list messages after chat failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to retrieve messages")
		return
	}
	conv.Messages = msgs

	// Find the last assistant message
	var assistantMsg *model.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgs[i]
			break
		}
	}

	usage := gin.H{
		"promptTokens":     0,
		"completionTokens": 0,
		"totalTokens":      0,
	}
	if assistantMsg != nil {
		usage["promptTokens"] = assistantMsg.TokensInput
		usage["completionTokens"] = assistantMsg.TokensOutput
		usage["totalTokens"] = assistantMsg.TokensInput + assistantMsg.TokensOutput
	}

	response.OK(c, gin.H{
		"message": assistantMsg,
		"chat":    conv,
		"usage":   usage,
	}, "message sent")
}

func (s *Server) handleChatMessageStream(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		c.Writer.Write([]byte("data: {\"error\":\"AUTH_ERROR\",\"message\":\"missing auth\"}\n\n"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusBadRequest)
		c.Writer.Write([]byte("data: {\"error\":\"STREAM_ERROR\",\"message\":\"invalid tenant\"}\n\n"))
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusBadRequest)
		c.Writer.Write([]byte("data: {\"error\":\"STREAM_ERROR\",\"message\":\"invalid user\"}\n\n"))
		return
	}

	var req struct {
		Content        string     `json:"content" binding:"required"`
		Model          string     `json:"model"`
		Context        string     `json:"context"`
		ConversationID *uuid.UUID `json:"conversationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusBadRequest)
		c.Writer.Write([]byte(fmt.Sprintf("data: {\"error\":\"STREAM_ERROR\",\"message\":%q}\n\n", err.Error())))
		return
	}

	ctx := c.Request.Context()
	streamCh, err := s.orch.Chat(ctx, tenantID, userID, req.ConversationID, req.Content, nil)
	if err != nil {
		s.log.Error("chat stream failed", logger.Error(err))
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Write([]byte(fmt.Sprintf("data: {\"error\":\"STREAM_ERROR\",\"message\":%q}\n\n", err.Error())))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	var conversationID string
	if req.ConversationID != nil {
		conversationID = req.ConversationID.String()
	}

	for chunk := range streamCh {
		data := fmt.Sprintf("data: {\"content\":%q,\"finished\":%v,\"conversationId\":%q}\n\n",
			chunk.Content, chunk.Done, conversationID)
		_, _ = c.Writer.Write([]byte(data))
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		if chunk.Done {
			break
		}
	}

	// Send final done event with conversation ID
	if conversationID == "" {
		// Find the most recent conversation
		convs, err := s.convRepo.ListByUser(ctx, tenantID, userID, 1, 0)
		if err == nil && len(convs) > 0 {
			conversationID = convs[0].ID.String()
		}
	}
	finalData := fmt.Sprintf("data: {\"content\":\"\",\"finished\":true,\"conversationId\":%q}\n\n", conversationID)
	_, _ = c.Writer.Write([]byte(finalData))
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
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

	multiClient.StartHealthChecks(ctx, 30*time.Second)

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

	// WebSocket manager
	wsManager := websocket.NewManager(orchestrator, log)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Health checker
	hc := health.NewChecker(pgPool, redisClient, nc)

	jwtMgr := jwt.NewManager(cfg.JWTSecret)
	srv := newServer(orchestrator, convStore, msgStore, artStore, fileHandler, log, multiClient, hc, providerHandler, wsManager)
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
