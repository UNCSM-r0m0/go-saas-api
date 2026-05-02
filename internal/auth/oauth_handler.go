package auth

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
)

// OAuthHandler provides HTTP handlers for OAuth flows
type OAuthHandler struct {
	oauthService *OAuthService
	log          logger.Logger
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(oauthService *OAuthService, log logger.Logger) *OAuthHandler {
	return &OAuthHandler{oauthService: oauthService, log: log}
}

// RegisterRoutes registers OAuth endpoints on the given router
func (h *OAuthHandler) RegisterRoutes(r *gin.Engine) {
	oauth := r.Group("/auth")
	{
		oauth.GET("/google", h.GoogleRedirect)
		oauth.GET("/google/callback", h.GoogleCallback)
		oauth.GET("/github", h.GitHubRedirect)
		oauth.GET("/github/callback", h.GitHubCallback)
		oauth.POST("/callback", h.GenericCallback)
	}
}

// GoogleRedirect redirects the user to Google's OAuth consent screen
func (h *OAuthHandler) GoogleRedirect(c *gin.Context) {
	url, err := h.oauthService.GetGoogleAuthURL(c.Request.Context())
	if err != nil {
		h.log.Error("google auth URL failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "oauth failed")
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback handles the OAuth callback from Google
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		response.Error(c, http.StatusBadRequest, "missing code")
		return
	}

	pair, _, err := h.oauthService.HandleGoogleCallback(c.Request.Context(), code, state)
	if err != nil {
		h.log.Error("google callback failed", logger.Error(err))
		response.Error(c, http.StatusUnauthorized, fmt.Sprintf("oauth callback failed: %v", err))
		return
	}

	setAuthCookies(c, pair)
	// Redirect to frontend after successful OAuth login
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusTemporaryRedirect, frontendURL)
}

// GitHubRedirect redirects the user to GitHub's OAuth consent screen
func (h *OAuthHandler) GitHubRedirect(c *gin.Context) {
	url, err := h.oauthService.GetGitHubAuthURL(c.Request.Context())
	if err != nil {
		h.log.Error("github auth URL failed", logger.Error(err))
		response.Error(c, http.StatusInternalServerError, "oauth failed")
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GitHubCallback handles the OAuth callback from GitHub
func (h *OAuthHandler) GitHubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		response.Error(c, http.StatusBadRequest, "missing code")
		return
	}

	pair, _, err := h.oauthService.HandleGitHubCallback(c.Request.Context(), code, state)
	if err != nil {
		h.log.Error("github callback failed", logger.Error(err))
		response.Error(c, http.StatusUnauthorized, "oauth callback failed")
		return
	}

	setAuthCookies(c, pair)
	// Redirect to frontend after successful OAuth login
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusTemporaryRedirect, frontendURL)
}

// GenericCallback handles OAuth callbacks in a unified way (POST).
// The frontend sends {code, state} and the backend tries Google first, then GitHub.
// In practice, the provider is inferred from the state token.
type genericCallbackReq struct {
	Code     string `json:"code" binding:"required"`
	State    string `json:"state" binding:"required"`
	Provider string `json:"provider"` // optional hint: "google" or "github"
}

func (h *OAuthHandler) GenericCallback(c *gin.Context) {
	var req genericCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var pair *TokenPair
	var user *User
	var err error

	// Try based on provider hint, or try both
	switch req.Provider {
	case "google":
		pair, user, err = h.oauthService.HandleGoogleCallback(c.Request.Context(), req.Code, req.State)
	case "github":
		pair, user, err = h.oauthService.HandleGitHubCallback(c.Request.Context(), req.Code, req.State)
	default:
		// Try Google first, then GitHub
		pair, user, err = h.oauthService.HandleGoogleCallback(c.Request.Context(), req.Code, req.State)
		if err != nil {
			pair, user, err = h.oauthService.HandleGitHubCallback(c.Request.Context(), req.Code, req.State)
		}
	}

	if err != nil {
		h.log.Error("oauth callback failed", logger.Error(err))
		response.Error(c, http.StatusUnauthorized, "oauth callback failed")
		return
	}

	setAuthCookies(c, pair)
	response.OK(c, gin.H{"user": user, "token": pair}, "oauth login successful")
}
