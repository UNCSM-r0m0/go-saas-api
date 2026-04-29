package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
)

// extractToken reads the JWT from either:
// 1. access_token cookie (HTTP-only, preferred for r3-chat frontend)
// 2. Authorization: Bearer <token> header (for API clients)
// Returns ("", false) if no auth present; ("", true) if auth is present but malformed.
func extractToken(c *gin.Context) (string, bool) {
	// 1. Try cookie first
	if token, err := c.Cookie("access_token"); err == nil && token != "" {
		return token, false
	}

	// 2. Fall back to Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return "", true // malformed
		}
		return parts[1], false
	}

	// 3. Final fallback: query param for WebSocket upgrades
	if strings.ToLower(c.GetHeader("Upgrade")) == "websocket" {
		if token := c.Query("token"); token != "" {
			return token, false
		}
	}

	return "", false
}

// Debug log helper — logs to stderr in dev so we can trace auth issues without
// requiring a logger dependency in this low-level middleware.
func debugAuthLog(c *gin.Context, msg string) {
	// Only log when ENV != production to avoid leaking info in prod.
	if os.Getenv("ENV") == "production" || os.Getenv("ENV") == "prod" {
		return
	}
	fmt.Fprintf(os.Stderr, "[gateway-auth-debug] %s | path=%s | cookie_present=%v | header_present=%v\n",
		msg,
		c.Request.URL.Path,
		c.GetHeader("Cookie") != "",
		c.GetHeader("Authorization") != "",
	)
}

// setAuthContext stores JWT claims in the Gin context and response headers.
func setAuthContext(c *gin.Context, claims *jwt.Claims) {
	c.Set("user_id", claims.UserID)
	c.Set("tenant_id", claims.TenantID)
	c.Set("role", claims.Role)
	c.Header("X-User-ID", claims.UserID)
	c.Header("X-Tenant-ID", claims.TenantID)
}

// JWTAuth creates a middleware that validates JWT tokens from cookies or headers.
func JWTAuth(jwtMgr *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, malformed := extractToken(c)
		if token == "" {
			debugAuthLog(c, "JWTAuth: no token found")
			if malformed {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
			}
			return
		}

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			debugAuthLog(c, fmt.Sprintf("JWTAuth: token validation failed: %v", err))
			if err == jwt.ErrExpiredToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		debugAuthLog(c, fmt.Sprintf("JWTAuth: success uid=%s", claims.UserID))
		setAuthContext(c, claims)
		c.Next()
	}
}

// JWTAuthOptional is like JWTAuth but allows unauthenticated requests.
func JWTAuthOptional(jwtMgr *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := extractToken(c)
		if token == "" {
			debugAuthLog(c, "JWTAuthOptional: no token, continuing as anonymous")
			c.Next()
			return
		}

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			debugAuthLog(c, fmt.Sprintf("JWTAuthOptional: token invalid (%v), continuing as anonymous", err))
			c.Next()
			return
		}

		debugAuthLog(c, fmt.Sprintf("JWTAuthOptional: success uid=%s", claims.UserID))
		setAuthContext(c, claims)
		c.Next()
	}
}
