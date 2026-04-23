package fileupload

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type memStore struct {
	uploads map[uuid.UUID]*Upload
}

func (m *memStore) Create(_ context.Context, u *Upload) error {
	m.uploads[u.ID] = u
	return nil
}

func (m *memStore) GetByID(_ context.Context, _, id uuid.UUID) (*Upload, error) {
	u, ok := m.uploads[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (m *memStore) ListByUser(_ context.Context, _, _ uuid.UUID, _, _ int) ([]Upload, error) {
	var list []Upload
	for _, u := range m.uploads {
		list = append(list, *u)
	}
	return list, nil
}

func (m *memStore) ListByConversation(_ context.Context, _, _ uuid.UUID) ([]Upload, error) {
	return nil, nil
}

func (m *memStore) Delete(_ context.Context, _, id uuid.UUID) error {
	delete(m.uploads, id)
	return nil
}

func TestSaveAndGet(t *testing.T) {
	store := &memStore{uploads: make(map[uuid.UUID]*Upload)}
	svc := NewService(store, t.TempDir(), 1024*1024)

	tenantID := uuid.New()
	userID := uuid.New()
	reader := strings.NewReader("hello world")

	upload, err := svc.Save(context.Background(), tenantID, userID, "test.txt", "text/plain", 11, reader)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if upload.OriginalName != "test.txt" {
		t.Fatalf("expected original_name test.txt, got %s", upload.OriginalName)
	}
	if upload.ContentType != "text/plain" {
		t.Fatalf("expected text/plain, got %s", upload.ContentType)
	}

	got, err := svc.Get(context.Background(), tenantID, upload.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.ID != upload.ID {
		t.Fatal("id mismatch")
	}
}

func TestReadText(t *testing.T) {
	store := &memStore{uploads: make(map[uuid.UUID]*Upload)}
	svc := NewService(store, t.TempDir(), 1024*1024)

	tenantID := uuid.New()
	userID := uuid.New()
	reader := strings.NewReader("file content")

	upload, err := svc.Save(context.Background(), tenantID, userID, "test.txt", "text/plain", 12, reader)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	text, err := svc.ReadText(upload)
	if err != nil {
		t.Fatalf("read text failed: %v", err)
	}
	if text != "file content" {
		t.Fatalf("expected 'file content', got %q", text)
	}
}

func TestDelete(t *testing.T) {
	store := &memStore{uploads: make(map[uuid.UUID]*Upload)}
	svc := NewService(store, t.TempDir(), 1024*1024)

	tenantID := uuid.New()
	userID := uuid.New()
	reader := strings.NewReader("hello")

	upload, err := svc.Save(context.Background(), tenantID, userID, "test.txt", "text/plain", 5, reader)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := svc.Delete(context.Background(), tenantID, upload.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = svc.Get(context.Background(), tenantID, upload.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestSizeLimit(t *testing.T) {
	store := &memStore{uploads: make(map[uuid.UUID]*Upload)}
	svc := NewService(store, t.TempDir(), 5)

	tenantID := uuid.New()
	userID := uuid.New()
	reader := strings.NewReader("hello world")

	_, err := svc.Save(context.Background(), tenantID, userID, "test.txt", "text/plain", 11, reader)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}
