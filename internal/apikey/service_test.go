package apikey

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memStore struct {
	keys map[string]*APIKey // by hash
}

func (m *memStore) Create(_ context.Context, key *APIKey) error {
	m.keys[key.KeyHash] = key
	return nil
}

func (m *memStore) GetByKeyHash(_ context.Context, hash string) (*APIKey, error) {
	return m.keys[hash], nil
}

func (m *memStore) ListByUser(_ context.Context, _, userID uuid.UUID) ([]APIKey, error) {
	var list []APIKey
	for _, k := range m.keys {
		if k.UserID == userID && k.RevokedAt == nil {
			list = append(list, *k)
		}
	}
	return list, nil
}

func (m *memStore) Revoke(_ context.Context, _, id uuid.UUID) error {
	for _, k := range m.keys {
		if k.ID == id {
			now := time.Now()
			k.RevokedAt = &now
		}
	}
	return nil
}

func (m *memStore) UpdateLastUsed(_ context.Context, _ uuid.UUID) error { return nil }

func TestGenerateKey(t *testing.T) {
	store := &memStore{keys: make(map[string]*APIKey)}
	svc := NewService(store)

	tenantID := uuid.New()
	userID := uuid.New()

	plainKey, key, err := svc.GenerateKey(context.Background(), tenantID, userID, "test-key", nil)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	if plainKey == "" {
		t.Fatal("expected non-empty plain key")
	}
	if key.Name != "test-key" {
		t.Fatalf("expected name test-key, got %s", key.Name)
	}
	if key.UserID != userID {
		t.Fatal("user_id mismatch")
	}
}

func TestValidateKey(t *testing.T) {
	store := &memStore{keys: make(map[string]*APIKey)}
	svc := NewService(store)

	tenantID := uuid.New()
	userID := uuid.New()

	plainKey, _, err := svc.GenerateKey(context.Background(), tenantID, userID, "test-key", nil)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	validated, err := svc.ValidateKey(context.Background(), plainKey)
	if err != nil {
		t.Fatalf("validate key failed: %v", err)
	}
	if validated.UserID != userID {
		t.Fatal("user_id mismatch after validation")
	}

	_, err = svc.ValidateKey(context.Background(), "invalid-key")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestRevokeKey(t *testing.T) {
	store := &memStore{keys: make(map[string]*APIKey)}
	svc := NewService(store)

	tenantID := uuid.New()
	userID := uuid.New()

	plainKey, key, err := svc.GenerateKey(context.Background(), tenantID, userID, "test-key", nil)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	if err := svc.RevokeKey(context.Background(), tenantID, key.ID); err != nil {
		t.Fatalf("revoke key failed: %v", err)
	}

	_, err = svc.ValidateKey(context.Background(), plainKey)
	if err == nil {
		t.Fatal("expected error for revoked key")
	}
}

func TestListKeys(t *testing.T) {
	store := &memStore{keys: make(map[string]*APIKey)}
	svc := NewService(store)

	tenantID := uuid.New()
	userID := uuid.New()

	_, _, err := svc.GenerateKey(context.Background(), tenantID, userID, "key-1", nil)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	_, _, err = svc.GenerateKey(context.Background(), tenantID, userID, "key-2", nil)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	keys, err := svc.ListKeys(context.Background(), tenantID, userID)
	if err != nil {
		t.Fatalf("list keys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}
