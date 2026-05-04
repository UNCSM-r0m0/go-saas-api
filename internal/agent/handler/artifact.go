package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

func (h *Handler) handleCreateArtifact(c *gin.Context) {
	var req struct {
		ConversationID uuid.UUID `json:"conversation_id" binding:"required"`
		Name           string    `json:"name" binding:"required"`
		Type           string    `json:"type" binding:"required"`
		Language       string    `json:"language"`
		Content        string    `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	art := &model.Artifact{
		ID:             uuid.New(),
		ConversationID: req.ConversationID,
		Name:           req.Name,
		Type:           req.Type,
		Language:       req.Language,
		Content:        req.Content,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.artRepo.Create(c.Request.Context(), art); err != nil {
		h.log.Error("create artifact failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to create artifact")
		return
	}

	response.Created(c, art, "artifact created")
}

func (h *Handler) handleGetArtifact(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	art, err := h.artRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("get artifact failed", logger.Error(err))
		response.Error(c, http.StatusNotFound, "artifact not found")
		return
	}

	response.OK(c, art, "artifact retrieved")
}

func (h *Handler) handleCreateWebsiteArtifact(c *gin.Context) {
	var req struct {
		ConversationID uuid.UUID `json:"conversation_id" binding:"required"`
		Content        string    `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	art := &model.Artifact{
		ID:             uuid.New(),
		ConversationID: req.ConversationID,
		Name:           "index.html",
		Type:           "website",
		Language:       "html",
		Content:        req.Content,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.artRepo.Create(c.Request.Context(), art); err != nil {
		h.log.Error("create website artifact failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to create artifact")
		return
	}

	response.Created(c, art, "artifact created")
}

func (h *Handler) handlePreviewArtifact(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	art, err := h.artRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("get artifact failed", logger.Error(err))
		response.Error(c, http.StatusNotFound, "artifact not found")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, art.Content)
}
