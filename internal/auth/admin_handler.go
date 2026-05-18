package auth

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

// AdminHandler provides HTTP handlers for admin user management
type AdminHandler struct {
	users UserRepository
	log   logger.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(users UserRepository, log logger.Logger) *AdminHandler {
	return &AdminHandler{users: users, log: log}
}

// RegisterRoutes registers admin endpoints on the given router
func (h *AdminHandler) RegisterRoutes(r *gin.Engine, authMiddleware, adminMiddleware gin.HandlerFunc) {
	admin := r.Group("/admin/users")
	admin.Use(authMiddleware, adminMiddleware)
	{
		admin.GET("", h.ListUsers)
		admin.POST("/:id/subscription", h.UpdateUserSubscription)
	}
}

// ListUsers handles GET /admin/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")

	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	users, err := h.users.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		h.log.Error("list users failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to list users")
		return
	}

	// Sanitize response (exclude password_hash)
	type userResponse struct {
		ID                    string `json:"id"`
		Email                 string `json:"email"`
		Name                  string `json:"name"`
		Role                  string `json:"role"`
		IsAdmin               bool   `json:"is_admin"`
		MessagesUsedThisMonth int    `json:"messages_used_this_month"`
		CreatedAt             string `json:"created_at"`
	}

	result := make([]userResponse, len(users))
	for i, u := range users {
		result[i] = userResponse{
			ID:                    u.ID.String(),
			Email:                 u.Email,
			Name:                  u.Name,
			Role:                  u.Role,
			IsAdmin:               u.IsAdmin,
			MessagesUsedThisMonth: u.MessagesUsedThisMonth,
			CreatedAt:             u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	response.OK(c, gin.H{"users": result}, "users retrieved")
}

// UpdateUserSubscription handles POST /admin/users/:id/subscription
func (h *AdminHandler) UpdateUserSubscription(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		Tier string `json:"tier" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate tier
	validTiers := map[string]bool{"free": true, "registered": true, "premium": true, "admin": true, "super_admin": true}
	if !validTiers[req.Tier] {
		response.Error(c, http.StatusBadRequest, "invalid tier")
		return
	}

	if err := h.users.UpdateUserRole(c.Request.Context(), userID, req.Tier); err != nil {
		h.log.Error("update user subscription failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to update user subscription")
		return
	}

	response.OK(c, gin.H{"tier": req.Tier}, "user subscription updated")
}
