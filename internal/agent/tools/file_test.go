package tools

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
)

type mockArtifactRepo struct {
	created *model.Artifact
	err     error
}

func (m *mockArtifactRepo) Create(_ context.Context, art *model.Artifact) error {
	m.created = art
	return m.err
}

func (m *mockArtifactRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.Artifact, error) {
	return nil, nil
}

func (m *mockArtifactRepo) ListByConversation(_ context.Context, _, _ uuid.UUID) ([]model.Artifact, error) {
	return nil, nil
}

// Compile-time check
var _ repository.ArtifactRepo = (*mockArtifactRepo)(nil)

func TestFileWriteTool_Execute(t *testing.T) {
	repo := &mockArtifactRepo{}
	tool := NewFileWriteTool(repo)

	ctx := context.WithValue(context.Background(), "tenant_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	ctx = context.WithValue(ctx, "conversation_id", uuid.MustParse("22222222-2222-2222-2222-222222222222"))

	res, err := tool.Execute(ctx, map[string]any{
		"name":     "index.html",
		"type":     "html",
		"language": "html",
		"content":  "<h1>Hello</h1>",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("unexpected tool error: %s", res.Error)
	}
	if repo.created == nil {
		t.Fatal("expected artifact to be created")
	}
	if repo.created.Name != "index.html" {
		t.Fatalf("expected name index.html, got %s", repo.created.Name)
	}
	if repo.created.Content != "<h1>Hello</h1>" {
		t.Fatalf("expected content <h1>Hello</h1>, got %s", repo.created.Content)
	}
}

func TestFileWriteTool_MissingName(t *testing.T) {
	repo := &mockArtifactRepo{}
	tool := NewFileWriteTool(repo)

	_, err := tool.Execute(context.Background(), map[string]any{
		"content": "foo",
	})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}
