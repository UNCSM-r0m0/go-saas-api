package tools

import (
	"context"
	"fmt"
	"strings"

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
	return "Read a file from the artifact store by name. If the file is not found in the current conversation, searches across all user conversations (workspace global). Args: name (string), conversation_id (optional UUID string)."
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
			"conversation_id": map[string]any{
				"type":        "string",
				"description": "Optional conversation ID to search in a specific conversation",
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

	// Try explicit conversation_id first
	if convIDStr, ok := args["conversation_id"].(string); ok && convIDStr != "" {
		if convID, err := uuid.Parse(convIDStr); err == nil {
			artifact, err := r.repo.GetByName(ctx, convID, name)
			if err == nil {
				return Result{Content: artifact.Content}, nil
			}
		}
	}

	// Try current conversation from context
	convID, _ := ctx.Value("conversation_id").(uuid.UUID)
	if convID != uuid.Nil {
		artifact, err := r.repo.GetByName(ctx, convID, name)
		if err == nil {
			return Result{Content: artifact.Content}, nil
		}
	}

	// Fallback: search across all user conversations (workspace global)
	userID, _ := ctx.Value("user_id").(uuid.UUID)
	if userID != uuid.Nil && r.repo != nil {
		arts, err := r.repo.ListByUser(ctx, userID)
		if err == nil {
			for _, art := range arts {
				if strings.EqualFold(art.Name, name) {
					return Result{Content: art.Content}, nil
				}
			}
		}
	}

	return Result{Error: fmt.Sprintf("file %q not found", name)}, fmt.Errorf("file not found")
}
