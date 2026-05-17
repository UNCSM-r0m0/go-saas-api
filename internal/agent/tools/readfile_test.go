package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
)

// mockReadFileRepo extends mockArtifactRepo with GetByName and ListByUser support.
type mockReadFileRepo struct {
	artifact    *model.Artifact
	userArtifacts []model.Artifact
	err         error
}

func (m *mockReadFileRepo) Create(_ context.Context, _ *model.Artifact) error {
	return nil
}

func (m *mockReadFileRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Artifact, error) {
	return nil, nil
}

func (m *mockReadFileRepo) GetByName(_ context.Context, _ uuid.UUID, _ string) (*model.Artifact, error) {
	return m.artifact, m.err
}

func (m *mockReadFileRepo) ListByConversation(_ context.Context, _ uuid.UUID) ([]model.Artifact, error) {
	return nil, nil
}

func (m *mockReadFileRepo) ListByUser(_ context.Context, _ uuid.UUID) ([]model.Artifact, error) {
	return m.userArtifacts, nil
}

func TestReadFileTool_Execute(t *testing.T) {
	expected := &model.Artifact{
		ID:             uuid.New(),
		ConversationID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Name:           "index.html",
		Content:        "<h1>Hello</h1>",
	}
	repo := &mockReadFileRepo{artifact: expected}
	tool := NewReadFileTool(repo)

	ctx := context.WithValue(context.Background(), "conversation_id", expected.ConversationID)

	res, err := tool.Execute(ctx, map[string]any{"name": "index.html"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("unexpected tool error: %s", res.Error)
	}
	if res.Content != expected.Content {
		t.Fatalf("expected content %q, got %q", expected.Content, res.Content)
	}
}

func TestReadFileTool_ExplicitConversationID(t *testing.T) {
	expected := &model.Artifact{
		ID:             uuid.New(),
		ConversationID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Name:           "style.css",
		Content:        "body { color: red; }",
	}
	repo := &mockReadFileRepo{artifact: expected}
	tool := NewReadFileTool(repo)

	// Pass a different conversation_id in context to prove explicit arg wins
	ctx := context.WithValue(context.Background(), "conversation_id", uuid.MustParse("44444444-4444-4444-4444-444444444444"))

	res, err := tool.Execute(ctx, map[string]any{
		"name":            "style.css",
		"conversation_id": "33333333-3333-3333-3333-333333333333",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != expected.Content {
		t.Fatalf("expected content %q, got %q", expected.Content, res.Content)
	}
}

func TestReadFileTool_WorkspaceGlobal(t *testing.T) {
	repo := &mockReadFileRepo{
		err: errors.New("not found in conversation"),
		userArtifacts: []model.Artifact{
			{
				ID:      uuid.New(),
				Name:    "global.css",
				Content: "/* global styles */",
			},
		},
	}
	tool := NewReadFileTool(repo)

	ctx := context.WithValue(context.Background(), "conversation_id", uuid.MustParse("22222222-2222-2222-2222-222222222222"))
	ctx = context.WithValue(ctx, "user_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))

	res, err := tool.Execute(ctx, map[string]any{"name": "global.css"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "/* global styles */" {
		t.Fatalf("expected global content, got %q", res.Content)
	}
}

func TestReadFileTool_NotFound(t *testing.T) {
	repo := &mockReadFileRepo{err: errors.New("artifact not found")}
	tool := NewReadFileTool(repo)

	ctx := context.WithValue(context.Background(), "conversation_id", uuid.MustParse("22222222-2222-2222-2222-222222222222"))
	ctx = context.WithValue(ctx, "user_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))

	res, err := tool.Execute(ctx, map[string]any{"name": "missing.html"})
	if err == nil {
		t.Fatal("expected error for missing artifact")
	}
	if res.Error == "" {
		t.Fatal("expected error message in result")
	}
}

func TestReadFileTool_MissingName(t *testing.T) {
	repo := &mockReadFileRepo{}
	tool := NewReadFileTool(repo)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

var _ repository.ArtifactRepo = (*mockReadFileRepo)(nil)
