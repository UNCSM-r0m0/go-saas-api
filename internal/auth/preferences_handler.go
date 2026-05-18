package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

// PreferencesHandler provides HTTP handlers for user preferences
type PreferencesHandler struct {
	store PreferencesRepository
	log   logger.Logger
}

// NewPreferencesHandler creates a new preferences handler
func NewPreferencesHandler(store PreferencesRepository, log logger.Logger) *PreferencesHandler {
	return &PreferencesHandler{store: store, log: log}
}

// RegisterRoutes registers preferences endpoints on the given router
func (h *PreferencesHandler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	prefs := r.Group("/users/preferences")
	prefs.Use(authMiddleware)
	{
		prefs.GET("", h.GetPreferences)
		prefs.PUT("", h.UpdatePreferences)
	}
}

// GetPreferences handles GET /users/preferences
func (h *PreferencesHandler) GetPreferences(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid user")
		return
	}

	prefs, err := h.store.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		// Return empty preferences if not found
		response.OK(c, gin.H{
			"display_name": "",
			"profession":   "",
			"traits":       []string{},
			"about_me":     "",
			"theme":        "system",
		}, "preferences retrieved")
		return
	}

	response.OK(c, prefs, "preferences retrieved")
}

// UpdatePreferences handles PUT /users/preferences
func (h *PreferencesHandler) UpdatePreferences(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid user")
		return
	}

	var req struct {
		DisplayName string   `json:"display_name"`
		Profession  string   `json:"profession"`
		Traits      []string `json:"traits"`
		AboutMe     string   `json:"about_me"`
		Theme       string   `json:"theme"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	prefs := &UserPreferences{
		ID:          uuid.New(),
		UserID:      userID,
		DisplayName: req.DisplayName,
		Profession:  req.Profession,
		Traits:      req.Traits,
		AboutMe:     req.AboutMe,
		Theme:       req.Theme,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := h.store.Upsert(c.Request.Context(), prefs); err != nil {
		h.log.Error("update preferences failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	response.OK(c, prefs, "preferences updated")
}
