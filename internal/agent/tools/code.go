package tools

import (
	"context"
	"fmt"
)

// SandboxClient executes code in an isolated environment.
type SandboxClient interface {
	Execute(ctx context.Context, code, language string) (string, error)
}

// CodeExecuteTool runs code via a sandbox client.
type CodeExecuteTool struct {
	client SandboxClient
}

// NewCodeExecuteTool creates a new code_execute tool.
func NewCodeExecuteTool(client SandboxClient) *CodeExecuteTool {
	return &CodeExecuteTool{client: client}
}

// Name returns the tool name.
func (c *CodeExecuteTool) Name() string { return "code_execute" }

// Description returns the tool description.
func (c *CodeExecuteTool) Description() string {
	return "Execute code in an isolated sandbox. Args: code (string), language (string)."
}

// Schema returns the JSON Schema for the tool's parameters.
func (c *CodeExecuteTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code":     map[string]any{"type": "string", "description": "Code to execute"},
			"language": map[string]any{"type": "string", "description": "Programming language (e.g., python, go, javascript)"},
		},
		"required": []string{"code"},
	}
}

// Execute runs code in the sandbox and returns the output.
func (c *CodeExecuteTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return Result{Error: "missing or invalid 'code' argument"}, fmt.Errorf("missing code")
	}
	language, _ := args["language"].(string)
	if language == "" {
		language = "python" // default
	}

	output, err := c.client.Execute(ctx, code, language)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	return Result{Content: output}, nil
}
