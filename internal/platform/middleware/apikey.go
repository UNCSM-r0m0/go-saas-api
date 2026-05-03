package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/apikey"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
)

// APIKeyAuth creates a middleware that validates X-API-Key headers.
// If a valid key is found, it sets user_id in the context.
// If no key is provided, it continues to the next handler (allowing JWT fallback).
func APIKeyAuth(service *apikey.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKeyHeader := c.GetHeader("X-API-Key")
		if apiKeyHeader == "" {
			c.Next()
			return
		}

		key, err := service.ValidateKey(c.Request.Context(), apiKeyHeader)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}

		c.Set("user_id", key.UserID.String())
		c.Set("role", "api_key")
		c.Header("X-User-ID", key.UserID.String())

		c.Next()
	}
}

// JWTOrAPIKeyAuth requires either a valid JWT (cookie or header) or a valid API key.
// It tries JWT first, then falls back to API key.
func JWTOrAPIKeyAuth(jwtMgr *jwt.Manager, apiKeyService *apikey.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try JWT first (cookie or header)
		token, malformed := extractToken(c)
		if token != "" {
			claims, err := jwtMgr.ValidateToken(token)
			if err == nil {
				setAuthContext(c, claims)
				c.Next()
				return
			}
		}

		// Try API key
		apiKeyHeader := c.GetHeader("X-API-Key")
		if apiKeyHeader != "" {
			key, err := apiKeyService.ValidateKey(c.Request.Context(), apiKeyHeader)
			if err == nil {
				c.Set("user_id", key.UserID.String())
				c.Set("role", "api_key")
				c.Header("X-User-ID", key.UserID.String())
				c.Next()
				return
			}
		}

		if malformed {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
	}
}
