package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
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
		Timezone       string      `json:"timezone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeSSEError(c, http.StatusBadRequest, "STREAM_ERROR", err.Error())
		return
	}

	ctx := c.Request.Context()

	// Obtener preferencias del usuario para personalizar el system prompt
	prefs, _ := getUserPreferences(ctx, h.pgPool, userID)
	var contextItems []model.UserContextItem
	if h.userCtxRepo != nil {
		contextItems, _ = h.userCtxRepo.GetByUser(ctx, userID)
	}
	userContext := buildUserContext(prefs, contextItems)

	// Inyectar contexto temporal determinista
	temporalCtx := runtime.BuildTemporalContext(req.Timezone)
	if userContext != "" {
		userContext = temporalCtx + "\n" + userContext
	} else {
		userContext = temporalCtx
	}

	convID, streamCh, err := h.orch.Chat(ctx, userID, req.ConversationID, req.Content, req.FileIDs, req.Model, userContext, req.Mode)
	if err != nil {
		h.log.Error("chat stream failed", logger.Error(err))
		writeSSEError(c, http.StatusOK, "STREAM_ERROR", err.Error())
		return
	}

	writeSSEHeaders(c)
	writeSSEAgentLoop(c, streamCh, convID.String())

	// Trigger background memory extraction after stream completes
	h.orch.ExtractMemory(ctx, userID, convID)
	// NOTE: writeSSEAgentLoop already sends finished:true when chunk.Done is received.
	// We only send a final fallback if the stream ended without a done event.
	// This prevents double finished:true events.
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
	var contextItems []model.UserContextItem
	if h.userCtxRepo != nil {
		contextItems, _ = h.userCtxRepo.GetByUser(ctx, userID)
	}
	userContext := buildUserContext(prefs, contextItems)

	convID, streamCh, err := h.orch.Chat(ctx, userID, req.ConversationID, req.Message, req.FileIDs, req.Model, userContext, req.Mode)
	if err != nil {
		h.log.Error("chat failed", logger.Error(err))
		writeSSEError(c, http.StatusInternalServerError, "STREAM_ERROR", "chat failed")
		return
	}

	writeSSEHeaders(c)
	writeSSEAgentLoop(c, streamCh, convID.String())

	// Trigger background memory extraction after stream completes
	h.orch.ExtractMemory(ctx, userID, convID)
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
		case "progress":
			data := fmt.Sprintf("data: {\"event\":\"progress\",\"content\":%q,\"conversationId\":%q}\n\n",
				chunk.Content, conversationID)
			_, _ = c.Writer.Write([]byte(data))
		case "artifact":
			data := fmt.Sprintf("data: {\"event\":\"artifact\",\"artifactId\":%q,\"artifactType\":%q,\"conversationId\":%q}\n\n",
				chunk.ArtifactID, chunk.ArtifactType, conversationID)
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
