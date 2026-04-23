package apikey

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
)

// Handler provides HTTP handlers for API key management.
type Handler struct {
	service *Service
	log     logger.Logger
}

// NewHandler creates a new API key handler.
func NewHandler(service *Service, log logger.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// RegisterRoutes registers API key endpoints on the given router.
func (h *Handler) RegisterRoutes(r *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	keys := r.Group("/api-keys")
	keys.Use(jwtMiddleware)
	{
		keys.POST("", h.Create)
		keys.GET("", h.List)
		keys.DELETE("/:id", h.Revoke)
	}
}

// Create handles POST /api-keys.
func (h *Handler) Create(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	tenantIDStr, _ := c.Get("tenant_id")
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plainKey, key, err := h.service.GenerateKey(c.Request.Context(), tenantID, userID, req.Name, nil)
	if err != nil {
		h.log.Error("generate api key failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create api key"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"key":       plainKey,
		"id":        key.ID,
		"name":      key.Name,
		"scopes":    key.Scopes,
		"created_at": key.CreatedAt,
	})
}

// List handles GET /api-keys.
func (h *Handler) List(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	tenantIDStr, _ := c.Get("tenant_id")
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant"})
		return
	}

	keys, err := h.service.ListKeys(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.log.Error("list api keys failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list api keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// Revoke handles DELETE /api-keys/:id.
func (h *Handler) Revoke(c *gin.Context) {
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.RevokeKey(c.Request.Context(), tenantID, id); err != nil {
		h.log.Error("revoke api key failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke api key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "api key revoked"})
}
