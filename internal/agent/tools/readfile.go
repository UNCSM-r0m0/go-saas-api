package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
)

// ReadFileTool reads a file from the artifact store by name.
type ReadFileTool struct {
	repo repository.ArtifactRepo
}

// NewReadFileTool creates a new read_file tool.
func NewReadFileTool(repo repository.ArtifactRepo) *ReadFileTool {
	return &ReadFileTool{repo: repo}
}

// Name returns the tool name.
func (r *ReadFileTool) Name() string { return "read_file" }

// Description returns the tool description.
func (r *ReadFileTool) Description() string {
	return "Read a file from the artifact store by name. Args: name (string)."
}

// Schema returns the JSON Schema for the tool's parameters.
func (r *ReadFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The name of the file to read",
			},
		},
		"required": []string{"name"},
	}
}

// Execute reads the file from the artifact store.
func (r *ReadFileTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return Result{Error: "missing or invalid 'name' argument"}, fmt.Errorf("missing name")
	}

	tenantID, _ := ctx.Value("tenant_id").(uuid.UUID)
	convID, _ := ctx.Value("conversation_id").(uuid.UUID)

	artifact, err := r.repo.GetByName(ctx, tenantID, convID, name)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	return Result{Content: artifact.Content}, nil
}
