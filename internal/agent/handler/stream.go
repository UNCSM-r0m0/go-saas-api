package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

func (h *Handler) handleChatMessageStream(c *gin.Context) {
	userID, ok := getUserIDSSE(c)
	if !ok {
		return
	}

	var req struct {
		Content        string      `json:"content" binding:"required"`
		Model          string      `json:"model"`
		Context        string      `json:"context"`
		ConversationID *uuid.UUID  `json:"conversationId"`
		FileIDs        []uuid.UUID `json:"fileIds"`
		Mode           string      `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeSSEError(c, http.StatusBadRequest, "STREAM_ERROR", err.Error())
		return
	}

	ctx := c.Request.Context()

	// Obtener preferencias del usuario para personalizar el system prompt
	prefs, _ := getUserPreferences(ctx, h.pgPool, userID)
	userContext := buildUserContext(prefs)

	streamCh, err := h.orch.Chat(ctx, userID, req.ConversationID, req.Content, req.FileIDs, req.Model, userContext, req.Mode)
	if err != nil {
		h.log.Error("chat stream failed", logger.Error(err))
		writeSSEError(c, http.StatusOK, "STREAM_ERROR", err.Error())
		return
	}

	writeSSEHeaders(c)

	var conversationID string
	if req.ConversationID != nil {
		conversationID = req.ConversationID.String()
	}

	writeSSEAgentLoop(c, streamCh, conversationID)

	if conversationID == "" {
		convs, err := h.convRepo.ListByUser(ctx, userID, 1, 0)
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

func (h *Handler) handleAgentChat(c *gin.Context) {
	userID, ok := getUserIDSSE(c)
	if !ok {
		return
	}

	var req struct {
		ConversationID *uuid.UUID  `json:"conversation_id"`
		Message        string      `json:"message" binding:"required"`
		Model          string      `json:"model"`
		FileIDs        []uuid.UUID `json:"file_ids"`
		Mode           string      `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeSSEError(c, http.StatusBadRequest, "STREAM_ERROR", err.Error())
		return
	}

	ctx := c.Request.Context()

	// Obtener preferencias del usuario para personalizar el system prompt
	prefs, _ := getUserPreferences(ctx, h.pgPool, userID)
	userContext := buildUserContext(prefs)

	streamCh, err := h.orch.Chat(ctx, userID, req.ConversationID, req.Message, req.FileIDs, req.Model, userContext, req.Mode)
	if err != nil {
		h.log.Error("chat failed", logger.Error(err))
		writeSSEError(c, http.StatusInternalServerError, "STREAM_ERROR", "chat failed")
		return
	}

	writeSSEHeaders(c)

	var conversationID string
	if req.ConversationID != nil {
		conversationID = req.ConversationID.String()
	}

	writeSSEAgentLoop(c, streamCh, conversationID)
}

func writeSSEAgentLoop(c *gin.Context, streamCh <-chan llm.Chunk, conversationID string) {
	for chunk := range streamCh {
		switch chunk.Event {
		case "tool_start":
			data := fmt.Sprintf("data: {\"event\":\"tool_start\",\"toolName\":%q,\"toolCallId\":%q,\"conversationId\":%q}\n\n",
				chunk.ToolName, chunk.ToolCall.ID, conversationID)
			_, _ = c.Writer.Write([]byte(data))
		case "tool_result":
			data := fmt.Sprintf("data: {\"event\":\"tool_result\",\"toolName\":%q,\"content\":%q,\"conversationId\":%q}\n\n",
				chunk.ToolName, chunk.Content, conversationID)
			_, _ = c.Writer.Write([]byte(data))
		case "artifact":
			data := fmt.Sprintf("data: {\"event\":\"artifact\",\"artifactId\":%q,\"artifactType\":\"website\",\"conversationId\":%q}\n\n",
				chunk.Content, conversationID)
			_, _ = c.Writer.Write([]byte(data))
		case "error":
			data := fmt.Sprintf("data: {\"event\":\"error\",\"content\":%q,\"conversationId\":%q}\n\n",
				chunk.Content, conversationID)
			_, _ = c.Writer.Write([]byte(data))
		default:
			// Regular content chunk
			if chunk.Content != "" || chunk.Done {
				data := fmt.Sprintf("data: {\"content\":%q,\"finished\":%v,\"conversationId\":%q}\n\n",
					chunk.Content, chunk.Done, conversationID)
				_, _ = c.Writer.Write([]byte(data))
			}
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		if chunk.Done {
			break
		}
	}
}
