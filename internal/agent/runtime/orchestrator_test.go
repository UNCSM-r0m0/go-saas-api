package runtime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// ---- mocks ----

type mockLLM struct {
	chunks []llm.Chunk
	err    error
}

func (m *mockLLM) Stream(_ context.Context, _ llm.Request) (<-chan llm.Chunk, error) {
	ch := make(chan llm.Chunk)
	go func() {
		defer close(ch)
		for _, c := range m.chunks {
			ch <- c
		}
	}()
	return ch, m.err
}

func (m *mockLLM) Complete(_ context.Context, _ llm.Request) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	var result string
	for _, c := range m.chunks {
		result += c.Content
	}
	return result, nil
}

func (m *mockLLM) HealthCheck(_ context.Context) error { return nil }

var _ llm.Client = (*mockLLM)(nil)

type memConversationRepo struct {
	convs map[uuid.UUID]*model.Conversation
}

func (m *memConversationRepo) Create(_ context.Context, conv *model.Conversation) error {
	m.convs[conv.ID] = conv
	return nil
}
func (m *memConversationRepo) GetByID(_ context.Context, _, id uuid.UUID) (*model.Conversation, error) {
	return m.convs[id], nil
}
func (m *memConversationRepo) ListByUser(_ context.Context, _, _ uuid.UUID, _, _ int) ([]model.Conversation, error) {
	return nil, nil
}
func (m *memConversationRepo) Update(_ context.Context, _ *model.Conversation) error { return nil }
func (m *memConversationRepo) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }

var _ repository.ConversationRepo = (*memConversationRepo)(nil)

type memMessageRepo struct {
	msgs []model.Message
}

func (m *memMessageRepo) Create(_ context.Context, msg *model.Message) error {
	m.msgs = append(m.msgs, *msg)
	return nil
}
func (m *memMessageRepo) ListByConversation(_ context.Context, _, _ uuid.UUID, _ int) ([]model.Message, error) {
	return m.msgs, nil
}

var _ repository.MessageRepo = (*memMessageRepo)(nil)

type memAgentRepo struct{}

func (m *memAgentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.Agent, error)  { return nil, nil }
func (m *memAgentRepo) GetByRole(_ context.Context, _ uuid.UUID, _ model.AgentRole) (*model.Agent, error) { return nil, nil }
func (m *memAgentRepo) GetDefault(_ context.Context, _ uuid.UUID) (*model.Agent, error) { return nil, nil }

var _ repository.AgentRepo = (*memAgentRepo)(nil)

type mockAgentRepoWithRole struct {
	agent *model.Agent
}

func (m *mockAgentRepoWithRole) GetByID(_ context.Context, _, _ uuid.UUID) (*model.Agent, error)  { return nil, nil }
func (m *mockAgentRepoWithRole) GetByRole(_ context.Context, _ uuid.UUID, _ model.AgentRole) (*model.Agent, error) { return m.agent, nil }
func (m *mockAgentRepoWithRole) GetDefault(_ context.Context, _ uuid.UUID) (*model.Agent, error) { return nil, nil }

var _ repository.AgentRepo = (*mockAgentRepoWithRole)(nil)

func TestOrchestrator_Chat(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "Hello! "},
			{Content: "How can I help?"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil)
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	ch, err := orch.Chat(ctx, tenantID, userID, nil, "hi there", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result string
	for chunk := range ch {
		result += chunk.Content
	}

	if result != "Hello! How can I help?" {
		t.Fatalf("unexpected result: %q", result)
	}

	// Assert messages saved
	if len(msgRepo.msgs) != 2 {
		t.Fatalf("expected 2 messages (user+assistant), got %d", len(msgRepo.msgs))
	}
	if msgRepo.msgs[0].Role != model.MessageRoleUser {
		t.Fatalf("expected first message user, got %s", msgRepo.msgs[0].Role)
	}
	if msgRepo.msgs[1].Role != model.MessageRoleAssistant {
		t.Fatalf("expected second message assistant, got %s", msgRepo.msgs[1].Role)
	}
}

func TestOrchestrator_NativeToolCall(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	_ = registry.Register(&mockTool{name: "echo", desc: "echo tool", result: tools.Result{Content: "pong"}})
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "I will echo that."},
			{Done: true, ToolCall: &llm.ToolCall{Name: "echo", Arguments: map[string]any{"msg": "ping"}}},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil)
	ctx := context.Background()

	ch, err := orch.Chat(ctx, uuid.New(), uuid.New(), nil, "ping", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result string
	for chunk := range ch {
		result += chunk.Content
	}

	if !contains(result, "pong") {
		t.Fatalf("expected tool result pong in output, got: %q", result)
	}

	// Assistant message should have tool call recorded
	if len(msgRepo.msgs) < 2 {
		t.Fatalf("expected messages saved")
	}
	assistantMsg := msgRepo.msgs[len(msgRepo.msgs)-1]
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].Name != "echo" {
		t.Fatalf("expected tool name echo, got %s", assistantMsg.ToolCalls[0].Name)
	}
}

func TestOrchestrator_ChatWithTool_MultipleChunks(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	_ = registry.Register(&mockTool{name: "echo", desc: "echo tool", result: tools.Result{Content: "pong"}})
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "I will echo that."},
			{Content: " "},
			{Done: true, ToolCall: &llm.ToolCall{Name: "echo", Arguments: map[string]any{"msg": "ping"}}},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil)
	ctx := context.Background()

	ch, err := orch.Chat(ctx, uuid.New(), uuid.New(), nil, "ping", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result string
	for chunk := range ch {
		result += chunk.Content
	}

	if !contains(result, "pong") {
		t.Fatalf("expected tool result pong in output, got: %q", result)
	}

	// Assistant message should have tool call recorded
	if len(msgRepo.msgs) < 2 {
		t.Fatalf("expected messages saved")
	}
	assistantMsg := msgRepo.msgs[len(msgRepo.msgs)-1]
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].Name != "echo" {
		t.Fatalf("expected tool name echo, got %s", assistantMsg.ToolCalls[0].Name)
	}
}

func TestOrchestrator_resolveAgent_DBLookup(t *testing.T) {
	expectedAgent := &model.Agent{
		ID:           uuid.New(),
		Name:         "custom-coder",
		Role:         model.RoleCoder,
		Model:        "gpt-4",
		SystemPrompt: "custom prompt",
	}
	agentRepo := &mockAgentRepoWithRole{agent: expectedAgent}
	orch := NewOrchestrator(nil, nil, nil, agentRepo, nil, nil)
	ctx := context.Background()
	tenantID := uuid.New()

	agent, err := orch.resolveAgent(ctx, tenantID, model.RoleCoder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}
	if agent.ID != expectedAgent.ID {
		t.Fatalf("expected agent ID %s, got %s", expectedAgent.ID, agent.ID)
	}
	if agent.Model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %s", agent.Model)
	}
}

func TestOrchestrator_resolveAgent_FallbackToDefault(t *testing.T) {
	agentRepo := &memAgentRepo{}
	orch := NewOrchestrator(nil, nil, nil, agentRepo, nil, nil)
	ctx := context.Background()
	tenantID := uuid.New()

	agent, err := orch.resolveAgent(ctx, tenantID, model.RoleCoder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected default agent, got nil")
	}
	if agent.Role != model.RoleCoder {
		t.Fatalf("expected role coder, got %s", agent.Role)
	}
	if agent.Model != "qwen2.5-coder:7b" {
		t.Fatalf("expected default model, got %s", agent.Model)
	}
	if agent.SystemPrompt == "" {
		t.Fatal("expected non-empty system prompt for coder role")
	}
}

