package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/runtime"
	"github.com/r0lm0/go-saas-api/internal/agent/websocket"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// Handler provides HTTP handlers for the agent service.
type Handler struct {
	orch       *runtime.Orchestrator
	convRepo   repository.ConversationRepo
	msgRepo    repository.MessageRepo
	artRepo    repository.ArtifactRepo
	log        logger.Logger
	llmManager *llm.MultiClient
	wsManager  *websocket.Manager
	hc         *health.Checker
}

// NewHandler creates a new agent handler.
func NewHandler(
	orch *runtime.Orchestrator,
	convRepo repository.ConversationRepo,
	msgRepo repository.MessageRepo,
	artRepo repository.ArtifactRepo,
	log logger.Logger,
	llmManager *llm.MultiClient,
	wsManager *websocket.Manager,
	hc *health.Checker,
) *Handler {
	return &Handler{
		orch:       orch,
		convRepo:   convRepo,
		msgRepo:    msgRepo,
		artRepo:    artRepo,
		log:        log,
		llmManager: llmManager,
		wsManager:  wsManager,
		hc:         hc,
	}
}

// RegisterRoutes registers all agent endpoints on the given router.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.handleHealthDetailed())
	r.GET("/chat/models", h.handleListModels)
	r.GET("/models/public", h.handleListModelsPublic)
	r.POST("/agent/chat", h.handleAgentChat)
	r.GET("/agent/ws", h.wsManager.HandleUpgrade)
	r.POST("/artifacts", h.handleCreateArtifact)
	r.GET("/artifacts/:id/preview", h.handlePreviewArtifact)

	// Conversations REST API (legacy)
	r.GET("/conversations", h.handleListConversations)
	r.GET("/conversations/:id", h.handleGetConversation)
	r.PATCH("/conversations/:id", h.handleUpdateConversation)
	r.DELETE("/conversations/:id", h.handleDeleteConversation)
	r.GET("/conversations/:id/messages", h.handleListMessages)

	// r3-chat frontend compatible chat routes
	r.GET("/chat/sessions", h.handleListConversations)
	r.POST("/chat", h.handleCreateChat)
	r.GET("/chat/:id", h.handleGetChatWithMessages)
	r.PATCH("/chat/sessions/:id", h.handleUpdateConversation)
	r.DELETE("/chat/sessions/:id", h.handleDeleteConversation)
	r.POST("/chat/message", h.handleChatMessage)
	r.POST("/chat/message/stream", h.handleChatMessageStream)
}

func (h *Handler) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "agent-service",
		"timestamp": time.Now().UTC(),
	})
}

func (h *Handler) handleHealthDetailed() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.hc == nil {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"service":   "agent-service",
				"timestamp": time.Now().UTC(),
			})
			return
		}
		report := h.hc.Check(c.Request.Context())
		if !report.Healthy {
			c.JSON(http.StatusServiceUnavailable, report)
			return
		}
		c.JSON(http.StatusOK, report)
	}
}

func (h *Handler) handleListModels(c *gin.Context) {
	providers := h.llmManager.ListProviders()
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
	response.OK(c, models, "models retrieved")
}

func (h *Handler) handleListModelsPublic(c *gin.Context) {
	providers := h.llmManager.ListProviders()
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

func (h *Handler) handleCreateChat(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
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
		UserID:    userID,
		Title:     req.Title,
		Status:    model.ConversationActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if req.Model != "" {
		conv.Metadata = map[string]any{"model": req.Model}
	}

	if err := h.convRepo.Create(c.Request.Context(), conv); err != nil {
		h.log.Error("create chat failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to create chat")
		return
	}

	response.Created(c, conv, "chat created")
}

func (h *Handler) handleGetChatWithMessages(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	conv, err := h.convRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("get chat failed", logger.Error(err))
		response.Error(c, http.StatusNotFound, "chat not found")
		return
	}

	msgs, err := h.msgRepo.ListByConversation(c.Request.Context(), id, 100)
	if err != nil {
		h.log.Error("list messages failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to load messages")
		return
	}

	conv.Messages = msgs
	response.OK(c, conv, "chat retrieved")
}

func (h *Handler) handleChatMessage(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	var req struct {
		Content        string      `json:"content" binding:"required"`
		Model          string      `json:"model"`
		Context        string      `json:"context"`
		ConversationID *uuid.UUID  `json:"conversationId"`
		FileIDs        []uuid.UUID `json:"fileIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	streamCh, err := h.orch.Chat(ctx, userID, req.ConversationID, req.Content, req.FileIDs, req.Model)
	if err != nil {
		h.log.Error("chat failed", logger.Error(err))
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
		convs, err := h.convRepo.ListByUser(ctx, userID, 1, 0)
		if err != nil || len(convs) == 0 {
			response.Error(c, http.StatusInternalServerError, "failed to retrieve conversation")
			return
		}
		convID = convs[0].ID
	}

	conv, err := h.convRepo.GetByID(ctx, convID)
	if err != nil {
		h.log.Error("get conversation after chat failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to retrieve conversation")
		return
	}

	msgs, err := h.msgRepo.ListByConversation(ctx, convID, 100)
	if err != nil {
		h.log.Error("list messages after chat failed", logger.Error(err))
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


