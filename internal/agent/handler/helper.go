package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

// getUserID extracts and validates the X-User-ID header from the request.
func getUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, 401, "missing X-User-ID")
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, 400, "invalid X-User-ID")
		return uuid.Nil, false
	}
	return userID, true
}
