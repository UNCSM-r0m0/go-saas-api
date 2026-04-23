package runtime

import (
	"context"
	"testing"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
)

type mockTool struct {
	name   string
	desc   string
	result tools.Result
	err    error
}

func (m mockTool) Name() string        { return m.name }
func (m mockTool) Description() string { return m.desc }
func (m mockTool) Execute(_ context.Context, _ map[string]any) (tools.Result, error) {
	return m.result, m.err
}

func TestBuildRequest(t *testing.T) {
	agent := &model.Agent{
		Name:         "TestAgent",
		Role:         model.RoleCoder,
		Model:        "qwen2.5-coder:7b",
		SystemPrompt: "You are a coder.",
	}

	history := []model.Message{
		{Role: model.MessageRoleUser, Content: "hi"},
		{Role: model.MessageRoleAssistant, Content: "hello"},
	}

	toolList := []tools.Tool{
		mockTool{name: "file_write", desc: "write files"},
	}

	req := BuildRequest(agent, history, "create html page", toolList)

	if req.Model != "qwen2.5-coder:7b" {
		t.Fatalf("expected model qwen2.5-coder:7b, got %s", req.Model)
	}
	if len(req.Messages) != 4 { // system + 2 history + user
		t.Fatalf("expected 4 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Fatalf("expected first message role system, got %s", req.Messages[0].Role)
	}
	if !contains(req.Messages[0].Content, "file_write") {
		t.Error("expected system prompt to mention file_write tool")
	}
	if req.Messages[3].Content != "create html page" {
		t.Fatalf("expected user message 'create html page', got %s", req.Messages[3].Content)
	}
	if !req.Stream {
		t.Error("expected stream to be true")
	}
}

func TestBuildRequest_NoTools(t *testing.T) {
	agent := &model.Agent{Role: model.RoleAssistant, Model: "default"}
	req := BuildRequest(agent, nil, "hello", nil)
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if contains(req.Messages[0].Content, "file_write") {
		t.Error("did not expect tool mention when no tools provided")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
