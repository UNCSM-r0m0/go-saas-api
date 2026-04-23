package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
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
	}
}

// GoogleRedirect redirects the user to Google's OAuth consent screen
func (h *OAuthHandler) GoogleRedirect(c *gin.Context) {
	url, err := h.oauthService.GetGoogleAuthURL(c.Request.Context())
	if err != nil {
		h.log.Error("google auth URL failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "oauth failed"})
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback handles the OAuth callback from Google
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}

	pair, user, err := h.oauthService.HandleGoogleCallback(c.Request.Context(), code, state)
	if err != nil {
		h.log.Error("google callback failed", logger.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "oauth callback failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"token": pair,
	})
}

// GitHubRedirect redirects the user to GitHub's OAuth consent screen
func (h *OAuthHandler) GitHubRedirect(c *gin.Context) {
	url, err := h.oauthService.GetGitHubAuthURL(c.Request.Context())
	if err != nil {
		h.log.Error("github auth URL failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "oauth failed"})
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GitHubCallback handles the OAuth callback from GitHub
func (h *OAuthHandler) GitHubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}

	pair, user, err := h.oauthService.HandleGitHubCallback(c.Request.Context(), code, state)
	if err != nil {
		h.log.Error("github callback failed", logger.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "oauth callback failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"token": pair,
	})
}
