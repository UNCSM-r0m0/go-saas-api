package runtime

import (
	"context"
	"strings"
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
func (m mockTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"msg": map[string]any{"type": "string"}},
	}
}
func (m mockTool) Execute(_ context.Context, _ map[string]any) (tools.Result, error) {
	return m.result, m.err
}

func TestBuildMessages(t *testing.T) {
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

	messages := BuildMessages(agent, history, "create html page", "", nil)
	toolDefs := BuildToolDefinitions([]tools.Tool{mockTool{name: "file_write", desc: "write files"}})

	if len(messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Fatalf("expected first message role system, got %s", messages[0].Role)
	}
	if !contains(messages[0].Content, "file_write") {
		t.Error("expected system prompt to mention file_write in tool discipline")
	}
	if messages[3].Content != "create html page" {
		t.Fatalf("expected user message 'create html page', got %s", messages[3].Content)
	}
	if len(toolDefs) != 1 {
		t.Fatalf("expected 1 tool definition, got %d", len(toolDefs))
	}
	if toolDefs[0].Function.Name != "file_write" {
		t.Fatalf("expected tool name file_write, got %s", toolDefs[0].Function.Name)
	}
}

func TestBuildMessages_ToolRoleMessages(t *testing.T) {
	agent := &model.Agent{
		Role:         model.RoleAssistant,
		SystemPrompt: "You are helpful.",
	}

	history := []model.Message{
		{Role: model.MessageRoleUser, Content: "write hello.py"},
		{Role: model.MessageRoleAssistant, Content: "Let me write that file.", ToolCalls: []model.ToolCall{{ID: "call_1", Name: "file_write", Arguments: map[string]any{"filename": "hello.py"}}}},
		{Role: model.MessageRoleTool, Content: "File written successfully", ToolCallID: "call_1", ToolName: "file_write"},
	}

	messages := BuildMessages(agent, history, "now run it", "", nil)

	if len(messages) != 5 {
		t.Fatalf("expected 5 messages (system + 3 history + user), got %d", len(messages))
	}
	if messages[1].Role != "user" {
		t.Fatalf("expected user message, got %s", messages[1].Role)
	}
	if messages[2].Role != "assistant" {
		t.Fatalf("expected assistant message, got %s", messages[2].Role)
	}
	if len(messages[2].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call in assistant message, got %d", len(messages[2].ToolCalls))
	}
	if messages[2].ToolCalls[0].ID != "call_1" {
		t.Fatalf("expected tool call ID call_1, got %s", messages[2].ToolCalls[0].ID)
	}
	if messages[3].Role != "tool" {
		t.Fatalf("expected tool message, got %s", messages[3].Role)
	}
	if messages[3].ToolCallID != "call_1" {
		t.Fatalf("expected tool_call_id call_1, got %s", messages[3].ToolCallID)
	}
	if messages[3].Name != "file_write" {
		t.Fatalf("expected tool name file_write, got %s", messages[3].Name)
	}
}

func TestBuildMessages_NoHistory(t *testing.T) {
	agent := &model.Agent{Role: model.RoleAssistant, Model: "default"}
	messages := BuildMessages(agent, nil, "hello", "", nil)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
}

func TestBuildSystemPrompt_AntiWebSearchForTemporal(t *testing.T) {
	agent := &model.Agent{
		Name:         "TestAgent",
		Role:         model.RoleAssistant,
		SystemPrompt: "You are helpful.",
	}
	prompt := BuildSystemPrompt(agent, nil, "")
	if !strings.Contains(prompt, "NEVER use web_search for questions about current date, day, or time") {
		t.Error("expected system prompt to contain anti-web_search rule for temporal questions")
	}
}

func TestBuildTemporalContext(t *testing.T) {
	ctx := BuildTemporalContext("America/New_York")
	if !strings.Contains(ctx, "Current date:") {
		t.Error("expected temporal context to contain 'Current date:'")
	}
	if !strings.Contains(ctx, "Current day:") {
		t.Error("expected temporal context to contain 'Current day:'")
	}
	if !strings.Contains(ctx, "Current time:") {
		t.Error("expected temporal context to contain 'Current time:'")
	}
	if !strings.Contains(ctx, "Timezone: America/New_York") {
		t.Error("expected temporal context to contain 'Timezone: America/New_York'")
	}
}

func TestBuildTemporalContext_InvalidTimezoneFallbacksToUTC(t *testing.T) {
	// Must not panic and must still produce output
	ctx := BuildTemporalContext("invalid-zone")
	if !strings.Contains(ctx, "Timezone: invalid-zone") {
		t.Error("expected temporal context to preserve the requested timezone label")
	}
	if !strings.Contains(ctx, "Current date:") {
		t.Error("expected temporal context to contain 'Current date:'")
	}
}

func TestBuildTemporalContext_EmptyTimezoneFallbacksToUTC(t *testing.T) {
	ctx := BuildTemporalContext("")
	if !strings.Contains(ctx, "Timezone: UTC") {
		t.Errorf("expected temporal context to fallback to UTC, got: %s", ctx)
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
