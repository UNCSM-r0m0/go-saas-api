package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
)

// FileWriteTool persists generated files as artifacts.
type FileWriteTool struct {
	repo repository.ArtifactRepo
}

// NewFileWriteTool creates a new file_write tool.
func NewFileWriteTool(repo repository.ArtifactRepo) *FileWriteTool {
	return &FileWriteTool{repo: repo}
}

// Name returns the tool name.
func (f *FileWriteTool) Name() string { return "file_write" }

// Description returns the tool description.
func (f *FileWriteTool) Description() string {
	return "Write a file with the given name, type, language, and content. Creates an artifact."
}

// Schema returns the JSON Schema for the tool's parameters.
func (f *FileWriteTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":     map[string]any{"type": "string", "description": "File name including extension"},
			"content":  map[string]any{"type": "string", "description": "File content"},
			"type":     map[string]any{"type": "string", "description": "File MIME type"},
			"language": map[string]any{"type": "string", "description": "Programming language"},
		},
		"required": []string{"name", "content"},
	}
}

// Execute writes the file to the artifact store.
func (f *FileWriteTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return Result{Error: "missing or invalid 'name' argument"}, fmt.Errorf("missing name")
	}
	content, _ := args["content"].(string)
	fileType, _ := args["type"].(string)
	language, _ := args["language"].(string)

	// Extract conversation_id from context (injected by runtime)
	convID, _ := ctx.Value("conversation_id").(uuid.UUID)

	art := &model.Artifact{
		ID:             uuid.New(),
		ConversationID: convID,
		Name:           name,
		Type:           fileType,
		Language:       language,
		Content:        content,
		Version:        1,
	}

	if err := f.repo.Create(ctx, art); err != nil {
		return Result{Error: err.Error()}, err
	}

	return Result{Content: fmt.Sprintf("Artifact %s created successfully", art.ID)}, nil
}
