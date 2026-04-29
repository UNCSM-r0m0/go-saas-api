package fileupload

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AllowedContentTypes defines permitted file types.
var AllowedContentTypes = map[string]bool{
	"text/plain":             true,
	"text/markdown":          true,
	"application/json":       true,
	"text/csv":               true,
	"text/html":              true,
	"text/css":               true,
	"application/javascript": true,
	"image/png":              true,
	"image/jpeg":             true,
	"image/gif":              true,
	"image/webp":             true,
	// Document types (extracted via document-service)
	"application/pdf":                                              true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
}

// Service provides business logic for file uploads.
type Service struct {
	store     Store
	basePath  string
	maxSize   int64
}

// NewService creates a new file upload service.
func NewService(store Store, basePath string, maxSize int64) *Service {
	return &Service{store: store, basePath: basePath, maxSize: maxSize}
}

// Save stores a file on disk and persists metadata.
func (s *Service) Save(ctx context.Context, tenantID, userID uuid.UUID, originalName string, contentType string, size int64, reader io.Reader) (*Upload, error) {
	if size > s.maxSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", size, s.maxSize)
	}

	ct := normalizeContentType(contentType, originalName)
	if !AllowedContentTypes[ct] {
		return nil, fmt.Errorf("content type not allowed: %s", ct)
	}

	id := uuid.New()
	tenantDir := filepath.Join(s.basePath, tenantID.String())
	if err := os.MkdirAll(tenantDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}

	storagePath := filepath.Join(tenantDir, id.String())
	file, err := os.Create(storagePath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, reader)
	if err != nil {
		os.Remove(storagePath)
		return nil, fmt.Errorf("write file: %w", err)
	}

	upload := &Upload{
		ID:           id,
		TenantID:     tenantID,
		UserID:       userID,
		Name:         id.String(),
		OriginalName: originalName,
		ContentType:  ct,
		SizeBytes:    written,
		StoragePath:  storagePath,
		Metadata:     map[string]any{},
		CreatedAt:    time.Now(),
	}

	if err := s.store.Create(ctx, upload); err != nil {
		os.Remove(storagePath)
		return nil, fmt.Errorf("persist upload: %w", err)
	}

	return upload, nil
}

// Get retrieves an upload and validates ownership.
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Upload, error) {
	return s.store.GetByID(ctx, tenantID, id)
}

// ListByUser lists uploads for a user.
func (s *Service) ListByUser(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]Upload, error) {
	return s.store.ListByUser(ctx, tenantID, userID, limit, offset)
}

// ListByConversation lists uploads for a conversation.
func (s *Service) ListByConversation(ctx context.Context, tenantID, conversationID uuid.UUID) ([]Upload, error) {
	return s.store.ListByConversation(ctx, tenantID, conversationID)
}

// Delete removes an upload from disk and database.
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	upload, err := s.store.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := os.Remove(upload.StoragePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove file: %w", err)
	}
	return s.store.Delete(ctx, tenantID, id)
}

// Open returns a ReadCloser for the upload content.
func (s *Service) Open(upload *Upload) (io.ReadCloser, error) {
	return os.Open(upload.StoragePath)
}

// ReadText reads the full content of a text file.
func (s *Service) ReadText(upload *Upload) (string, error) {
	if !strings.HasPrefix(upload.ContentType, "text/") && upload.ContentType != "application/json" && upload.ContentType != "application/javascript" {
		return "", fmt.Errorf("not a text file")
	}
	data, err := os.ReadFile(upload.StoragePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeContentType(ct, filename string) string {
	if ct != "" && ct != "application/octet-stream" {
		return ct
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".go", ".py", ".rs", ".java", ".c", ".cpp", ".h":
		return "text/plain"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	if detected := mime.TypeByExtension(ext); detected != "" {
		return detected
	}
	return ct
}
