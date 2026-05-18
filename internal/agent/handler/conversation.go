package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

func (h *Handler) handleListConversations(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
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

	convs, err := h.convRepo.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("list conversations failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	response.OK(c, convs, "conversations retrieved")
}

func (h *Handler) handleGetConversation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	conv, err := h.convRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("get conversation failed", logger.Error(err))
		response.Error(c, http.StatusNotFound, "conversation not found")
		return
	}

	response.OK(c, conv, "conversation retrieved")
}

func (h *Handler) handleUpdateConversation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	conv, err := h.convRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("get conversation for update failed", logger.Error(err))
		response.Error(c, http.StatusNotFound, "conversation not found")
		return
	}

	var req struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Title != "" {
		conv.Title = req.Title
	}
	if req.Status != "" {
		conv.Status = model.ConversationStatus(req.Status)
	}
	conv.UpdatedAt = time.Now()

	if err := h.convRepo.Update(c.Request.Context(), conv); err != nil {
		h.log.Error("update conversation failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to update conversation")
		return
	}

	response.OK(c, conv, "conversation updated")
}

func (h *Handler) handleDeleteConversation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.convRepo.Delete(c.Request.Context(), id); err != nil {
		h.log.Error("delete conversation failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to delete conversation")
		return
	}

	response.OK(c, nil, "conversation deleted")
}

func (h *Handler) handleListMessages(c *gin.Context) {
	conversationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	msgs, err := h.msgRepo.ListByConversation(c.Request.Context(), conversationID, limit)
	if err != nil {
		h.log.Error("list messages failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to list messages")
		return
	}

	response.OK(c, msgs, "messages retrieved")
}
