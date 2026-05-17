package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
	"github.com/stretchr/testify/assert"
)

func TestWSAuth_ValidCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := jwt.NewManager("test-secret")

	// Simulate JWTOrAPIKeyAuth + proxyTo behavior for WS
	r := gin.New()
	r.GET("/agent/ws", func(c *gin.Context) {
		// Simulate auth middleware behavior
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		claims, err := mgr.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		// Simulate WS upgrade (would be 101 in real scenario)
		c.Status(http.StatusSwitchingProtocols)
	})

	token, _ := mgr.GenerateToken("user-123", "admin", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agent/ws", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSwitchingProtocols, w.Code)
}

func TestWSAuth_MissingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := jwt.NewManager("test-secret")

	r := gin.New()
	r.GET("/agent/ws", func(c *gin.Context) {
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			// Check Authorization header fallback
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
		}
		claims, err := mgr.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Status(http.StatusSwitchingProtocols)
	})

	// No cookie, no header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agent/ws", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "authentication required")
}

func TestWSAuth_FallbackToHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := jwt.NewManager("test-secret")

	r := gin.New()
	r.GET("/agent/ws", func(c *gin.Context) {
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
			// Parse Bearer token
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token = authHeader[7:]
			}
		}
		claims, err := mgr.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Status(http.StatusSwitchingProtocols)
	})

	token, _ := mgr.GenerateToken("user-123", "admin", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agent/ws", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSwitchingProtocols, w.Code)
}

func TestWSAuth_QueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := jwt.NewManager("test-secret")

	r := gin.New()
	r.GET("/agent/ws", func(c *gin.Context) {
		// Simulate extractToken behavior with query param fallback
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				if c.GetHeader("Upgrade") == "websocket" {
					token = c.Query("token")
				}
			}
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		claims, err := mgr.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Status(http.StatusSwitchingProtocols)
	})

	token, _ := mgr.GenerateToken("user-123", "admin", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agent/ws?token="+token, nil)
	req.Header.Set("Upgrade", "websocket")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSwitchingProtocols, w.Code)
}

func TestWSAuth_InvalidQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := jwt.NewManager("test-secret")

	r := gin.New()
	r.GET("/agent/ws", func(c *gin.Context) {
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				if c.GetHeader("Upgrade") == "websocket" {
					token = c.Query("token")
				}
			}
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		claims, err := mgr.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Status(http.StatusSwitchingProtocols)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agent/ws?token=invalid-token", nil)
	req.Header.Set("Upgrade", "websocket")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// closeNotifierRecorder wraps httptest.ResponseRecorder to implement http.CloseNotifier
// required by httputil.ReverseProxy in some code paths.
type closeNotifierRecorder struct {
	*httptest.ResponseRecorder
	closeCh chan bool
}

func newCloseNotifierRecorder() *closeNotifierRecorder {
	return &closeNotifierRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeCh:          make(chan bool, 1),
	}
}

func (c *closeNotifierRecorder) CloseNotify() <-chan bool {
	return c.closeCh
}

func TestForgotPasswordRoute_Proxied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Simulate auth-service handler
	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/forgot-password" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"reset email sent"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	authServer := httptest.NewServer(authHandler)
	defer authServer.Close()

	gateway := gin.New()
	gateway.POST("/auth/forgot-password", proxyTo(authServer.URL, ""))

	w := newCloseNotifierRecorder()
	req, _ := http.NewRequest("POST", "/auth/forgot-password", nil)
	gateway.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "reset email sent")
}

func TestResetPasswordRoute_Proxied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/reset-password" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"password updated"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	authServer := httptest.NewServer(authHandler)
	defer authServer.Close()

	gateway := gin.New()
	gateway.POST("/auth/reset-password", proxyTo(authServer.URL, ""))

	w := newCloseNotifierRecorder()
	req, _ := http.NewRequest("POST", "/auth/reset-password", nil)
	gateway.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "password updated")
}
