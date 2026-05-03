package provider

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
)

// Handler provides HTTP handlers for AI provider management.
type Handler struct {
	service *Service
	log     logger.Logger
}

// NewHandler creates a new provider handler.
func NewHandler(service *Service, log logger.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// RegisterRoutes registers provider admin endpoints.
func (h *Handler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	admin := r.Group("/admin/providers")
	admin.Use(authMiddleware)
	{
		admin.POST("", h.CreateProvider)
		admin.GET("", h.ListProviders)
		admin.GET("/:id", h.GetProvider)
		admin.PATCH("/:id", h.UpdateProvider)
		admin.DELETE("/:id", h.DeleteProvider)
		admin.POST("/:id/test", h.TestProvider)
		admin.POST("/:id/sync-models", h.SyncModels)
		admin.POST("/:id/models", h.CreateModel)
		admin.GET("/:id/models", h.ListModels)
	}

	models := r.Group("/admin/models")
	models.Use(authMiddleware)
	{
		models.GET("/:id", h.GetModel)
		models.PATCH("/:id", h.UpdateModel)
		models.DELETE("/:id", h.DeleteModel)
	}
}

func getUser(c *gin.Context) (userID uuid.UUID, ok bool) {
	userIDStr, _ := c.Get("user_id")
	if userIDStr == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing auth context"})
		return uuid.UUID{}, false
	}
	var err error
	userID, err = uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return uuid.UUID{}, false
	}
	return userID, true
}

// CreateProvider handles POST /admin/providers.
func (h *Handler) CreateProvider(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	var req struct {
		Name     string       `json:"name" binding:"required"`
		Type     ProviderType `json:"type" binding:"required"`
		BaseURL  string       `json:"base_url" binding:"required"`
		APIKey   string       `json:"api_key"`
		Priority int          `json:"priority"`
		IsPublic bool         `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.service.CreateProvider(c.Request.Context(), req.Name, req.Type, req.BaseURL, req.APIKey, req.Priority, req.IsPublic)
	if err != nil {
		h.log.Error("create provider failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create provider"})
		return
	}

	c.JSON(http.StatusCreated, p)
}

// ListProviders handles GET /admin/providers.
func (h *Handler) ListProviders(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	providers, err := h.service.ListProviders(c.Request.Context())
	if err != nil {
		h.log.Error("list providers failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list providers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

// GetProvider handles GET /admin/providers/:id.
func (h *Handler) GetProvider(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	p, err := h.service.GetProvider(c.Request.Context(), id)
	if err != nil {
		h.log.Error("get provider failed", logger.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}

	c.JSON(http.StatusOK, p)
}

// UpdateProvider handles PATCH /admin/providers/:id.
func (h *Handler) UpdateProvider(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Name     string       `json:"name" binding:"required"`
		Type     ProviderType `json:"type" binding:"required"`
		BaseURL  string       `json:"base_url" binding:"required"`
		APIKey   string       `json:"api_key"`
		Priority int          `json:"priority"`
		IsActive bool         `json:"is_active"`
		IsPublic bool         `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.service.UpdateProvider(c.Request.Context(), id, req.Name, req.Type, req.BaseURL, req.APIKey, req.Priority, req.IsActive, req.IsPublic)
	if err != nil {
		h.log.Error("update provider failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update provider"})
		return
	}

	c.JSON(http.StatusOK, p)
}

// DeleteProvider handles DELETE /admin/providers/:id.
func (h *Handler) DeleteProvider(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.DeleteProvider(c.Request.Context(), id); err != nil {
		h.log.Error("delete provider failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete provider"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "provider deleted"})
}

// SyncModels handles POST /admin/providers/:id/sync-models.
func (h *Handler) SyncModels(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	result, err := h.service.SyncModels(c.Request.Context(), id)
	if err != nil {
		h.log.Error("sync ollama models failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// TestProvider handles POST /admin/providers/:id/test.
func (h *Handler) TestProvider(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.TestProviderConnection(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CreateModel handles POST /admin/providers/:id/models.
func (h *Handler) CreateModel(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	var req struct {
		Name              string `json:"name" binding:"required"`
		DisplayName       string `json:"display_name" binding:"required"`
		Description       string `json:"description"`
		MaxTokens         int    `json:"max_tokens"`
		ContextWindow     int    `json:"context_window"`
		SupportsStreaming bool   `json:"supports_streaming"`
		SupportsImages    bool   `json:"supports_images"`
		IsPublic          bool   `json:"is_public"`
		IsPremium         bool   `json:"is_premium"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.service.CreateModel(c.Request.Context(), providerID, req.Name, req.DisplayName, req.Description, req.MaxTokens, req.ContextWindow, req.SupportsStreaming, req.SupportsImages, req.IsPublic, req.IsPremium)
	if err != nil {
		h.log.Error("create model failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create model"})
		return
	}

	c.JSON(http.StatusCreated, m)
}

// ListModels handles GET /admin/providers/:id/models.
func (h *Handler) ListModels(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}

	models, err := h.service.ListModelsByProvider(c.Request.Context(), providerID)
	if err != nil {
		h.log.Error("list models failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list models"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// GetModel handles GET /admin/models/:id.
func (h *Handler) GetModel(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	m, err := h.service.GetModel(c.Request.Context(), id)
	if err != nil {
		h.log.Error("get model failed", logger.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}

	c.JSON(http.StatusOK, m)
}

// UpdateModel handles PATCH /admin/models/:id.
func (h *Handler) UpdateModel(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Name              string `json:"name" binding:"required"`
		DisplayName       string `json:"display_name" binding:"required"`
		Description       string `json:"description"`
		MaxTokens         int    `json:"max_tokens"`
		ContextWindow     int    `json:"context_window"`
		SupportsStreaming bool   `json:"supports_streaming"`
		SupportsImages    bool   `json:"supports_images"`
		IsActive          bool   `json:"is_active"`
		IsPublic          bool   `json:"is_public"`
		IsPremium         bool   `json:"is_premium"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var descPtr *string
	if req.Description != "" {
		descPtr = &req.Description
	}
	m := &AIModel{
		ID:                id,
		Name:              req.Name,
		DisplayName:       req.DisplayName,
		Description:       descPtr,
		MaxTokens:         req.MaxTokens,
		ContextWindow:     req.ContextWindow,
		SupportsStreaming: req.SupportsStreaming,
		SupportsImages:    req.SupportsImages,
		IsActive:          req.IsActive,
		IsPublic:          req.IsPublic,
		IsPremium:         req.IsPremium,
	}

	if err := h.service.UpdateModel(c.Request.Context(), m); err != nil {
		h.log.Error("update model failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update model"})
		return
	}

	c.JSON(http.StatusOK, m)
}

// DeleteModel handles DELETE /admin/models/:id.
func (h *Handler) DeleteModel(c *gin.Context) {
	_, ok := getUser(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.DeleteModel(c.Request.Context(), id); err != nil {
		h.log.Error("delete model failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete model"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "model deleted"})
}
