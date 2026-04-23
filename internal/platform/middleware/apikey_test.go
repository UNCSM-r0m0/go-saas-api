package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/apikey"
)

type mockAPIKeyStore struct {
	keys map[string]*apikey.APIKey // by hash
}

func (m *mockAPIKeyStore) Create(_ context.Context, key *apikey.APIKey) error {
	m.keys[key.KeyHash] = key
	return nil
}

func (m *mockAPIKeyStore) GetByKeyHash(_ context.Context, hash string) (*apikey.APIKey, error) {
	return m.keys[hash], nil
}

func (m *mockAPIKeyStore) ListByUser(_ context.Context, _, _ uuid.UUID) ([]apikey.APIKey, error) {
	return nil, nil
}

func (m *mockAPIKeyStore) Revoke(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (m *mockAPIKeyStore) UpdateLastUsed(_ context.Context, _ uuid.UUID) error { return nil }

func TestAPIKeyAuth_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockAPIKeyStore{keys: make(map[string]*apikey.APIKey)}
	svc := apikey.NewService(store)

	tenantID := uuid.New()
	userID := uuid.New()
	plainKey, _, err := svc.GenerateKey(context.Background(), tenantID, userID, "test", nil)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	r := gin.New()
	r.Use(APIKeyAuth(svc))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":   c.GetString("user_id"),
			"tenant_id": c.GetString("tenant_id"),
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", plainKey)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockAPIKeyStore{keys: make(map[string]*apikey.APIKey)}
	svc := apikey.NewService(store)

	r := gin.New()
	r.Use(APIKeyAuth(svc))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyAuth_NoKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockAPIKeyStore{keys: make(map[string]*apikey.APIKey)}
	svc := apikey.NewService(store)

	r := gin.New()
	r.Use(APIKeyAuth(svc))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (continued to next handler), got %d", w.Code)
	}
}
