package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
	"github.com/stretchr/testify/assert"
)

func setupJWTRouter() (*gin.Engine, *jwt.Manager) {
	gin.SetMode(gin.TestMode)
	mgr := jwt.NewManager("test-secret")
	r := gin.New()
	return r, mgr
}

func TestJWTAuth_ValidToken(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/protected", JWTAuth(mgr), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": c.GetString("user_id"),
			"role":    c.GetString("role"),
		})
	})

	token, _ := mgr.GenerateToken("user-123", "admin", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "user-123")
	assert.Contains(t, w.Body.String(), "admin")
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/protected", JWTAuth(mgr), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing authorization")
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/protected", JWTAuth(mgr), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid authorization")
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/protected", JWTAuth(mgr), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	token, _ := mgr.GenerateToken("user-123", "admin", -time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}

func TestJWTAuthOptional_ValidToken(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/optional", JWTAuthOptional(mgr), func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})

	token, _ := mgr.GenerateToken("user-789", "member", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "user-789")
}

func TestJWTAuthOptional_NoToken(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/optional", JWTAuthOptional(mgr), func(c *gin.Context) {
		uid, exists := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"user_id": uid, "exists": exists})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/optional", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "false")
}

func TestExtractToken_WebSocketQueryParam(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/ws", JWTAuth(mgr), func(c *gin.Context) {
		c.Status(http.StatusSwitchingProtocols)
	})

	token, _ := mgr.GenerateToken("user-ws", "member", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws?token="+token, nil)
	req.Header.Set("Upgrade", "websocket")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSwitchingProtocols, w.Code)
}

func TestExtractToken_QueryParamWithoutUpgradeHeader(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/ws", JWTAuth(mgr), func(c *gin.Context) {
		c.Status(http.StatusSwitchingProtocols)
	})

	token, _ := mgr.GenerateToken("user-ws", "member", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws?token="+token, nil)
	// No Upgrade header
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing authorization")
}

func TestExtractToken_InvalidWebSocketQueryParam(t *testing.T) {
	r, mgr := setupJWTRouter()
	r.GET("/ws", JWTAuth(mgr), func(c *gin.Context) {
		c.Status(http.StatusSwitchingProtocols)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws?token=invalid-token", nil)
	req.Header.Set("Upgrade", "websocket")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid token")
}
