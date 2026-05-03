package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

func getUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing X-User-ID")
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid X-User-ID")
		return uuid.Nil, false
	}
	return userID, true
}

func getUserIDSSE(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		writeSSEError(c, http.StatusUnauthorized, "AUTH_ERROR", "missing auth")
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeSSEError(c, http.StatusBadRequest, "STREAM_ERROR", "invalid user")
		return uuid.Nil, false
	}
	return userID, true
}

func writeSSEError(c *gin.Context, status int, code string, msg string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.WriteHeader(status)
	c.Writer.Write([]byte(fmt.Sprintf("data: {\"error\":\"%s\",\"message\":\"%s\"}\n\n", code, msg)))
}

func writeSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)
}
