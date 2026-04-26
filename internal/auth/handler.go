package auth

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

// Handler provides HTTP handlers for authentication
type Handler struct {
	service *Service
	log     logger.Logger
}

// NewHandler creates a new auth handler
func NewHandler(service *Service, log logger.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// cookieConfig returns the appropriate cookie settings based on environment.
func cookieConfig() (secure bool, sameSite string) {
	if os.Getenv("ENV") == "production" || os.Getenv("ENV") == "prod" {
		return true, "lax"
	}
	return false, "lax"
}

// setAuthCookies sets the access_token and refresh_token cookies.
func setAuthCookies(c *gin.Context, pair *TokenPair) {
	secure, sameSite := cookieConfig()
	c.SetSameSite(sameSiteValue(sameSite))
	c.SetCookie("access_token", pair.AccessToken, int(pair.ExpiresIn), "/", "", secure, true)
	c.SetCookie("refresh_token", pair.RefreshToken, 7*24*60*60, "/", "", secure, true)
}

// clearAuthCookies clears the auth cookies.
func clearAuthCookies(c *gin.Context) {
	secure, sameSite := cookieConfig()
	c.SetSameSite(sameSiteValue(sameSite))
	c.SetCookie("access_token", "", -1, "/", "", secure, true)
	c.SetCookie("refresh_token", "", -1, "/", "", secure, true)
}

func sameSiteValue(v string) http.SameSite {
	switch v {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// RegisterRoutes registers auth endpoints on the given router
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/logout", h.Logout)
		auth.POST("/forgot-password", h.ForgotPassword)
		auth.POST("/reset-password", h.ResetPassword)
		// /me and /profile are registered separately with JWT middleware in main.go
	}
}

// Register handles user registration
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// For now, use a default tenant; in production this comes from signup context
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user, err := h.service.Register(c.Request.Context(), tenantID, &req)
	if err != nil {
		if err == ErrUserExists {
			response.Error(c, http.StatusConflict, "user already exists")
			return
		}
		h.log.Error("register failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "registration failed")
		return
	}

	// Generate token pair and set cookies
	pair, _, err := h.service.Login(c.Request.Context(), tenantID, &LoginRequest{Email: req.Email, Password: req.Password})
	if err != nil {
		h.log.Error("auto-login after register failed", logger.Error(err))
		response.OK(c, gin.H{"user": user}, "registered successfully")
		return
	}

	setAuthCookies(c, pair)
	response.OK(c, gin.H{"user": user, "token": pair}, "registered successfully")
}

// Login handles user login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	pair, user, err := h.service.Login(c.Request.Context(), tenantID, &req)
	if err != nil {
		if err == ErrInvalidCredentials {
			response.Error(c, http.StatusUnauthorized, "invalid credentials")
			return
		}
		h.log.Error("login failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "login failed")
		return
	}

	setAuthCookies(c, pair)
	response.OK(c, gin.H{"user": user, "token": pair}, "login successful")
}

// Refresh handles token refresh
func (h *Handler) Refresh(c *gin.Context) {
	// Try cookie first, then body
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		refreshToken = req.RefreshToken
	}

	pair, user, err := h.service.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	setAuthCookies(c, pair)
	response.OK(c, gin.H{"user": user, "token": pair}, "token refreshed")
}

// Logout handles user logout
func (h *Handler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie("refresh_token")
	if refreshToken == "" {
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}

	if refreshToken != "" {
		if err := h.service.Logout(c.Request.Context(), refreshToken); err != nil {
			h.log.Error("logout failed", logger.Error(err))
		}
	}

	clearAuthCookies(c)
	response.OK(c, nil, "logged out")
}

// ForgotPassword handles password reset requests
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// Always return the same response to avoid user enumeration
	_ = h.service.RequestPasswordReset(c.Request.Context(), tenantID, req.Email)

	response.OK(c, nil, "if the email exists, a reset link has been sent")
}

// ResetPassword handles password reset confirmation
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		if err == ErrInvalidResetToken {
			response.Error(c, http.StatusBadRequest, "invalid or expired reset token")
			return
		}
		h.log.Error("reset password failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "password reset failed")
		return
	}

	response.OK(c, nil, "password reset successful")
}

// Me returns the current authenticated user
func (h *Handler) Me(c *gin.Context) {
	// Expect userID set by middleware; if missing, unauthorized
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

	user, err := h.service.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	}

	response.OK(c, user, "user profile")
}

// UpdateProfile updates the authenticated user's profile.
func (h *Handler) UpdateProfile(c *gin.Context) {
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

	user, err := h.service.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	}

	var req struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	// Avatar is not stored in DB yet; extend User model if needed
	_ = req.Avatar

	user.UpdatedAt = time.Now().UTC()
	if err := h.service.users.Update(c.Request.Context(), user); err != nil {
		h.log.Error("update profile failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "failed to update profile")
		return
	}

	response.OK(c, user, "profile updated")
}
