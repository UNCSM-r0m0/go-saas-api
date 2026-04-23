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

func (m *memAgentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.Agent, error) { return nil, nil }
func (m *memAgentRepo) GetDefault(_ context.Context, _ uuid.UUID) (*model.Agent, error) { return nil, nil }

var _ repository.AgentRepo = (*memAgentRepo)(nil)

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

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil)
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

func TestParseToolCall(t *testing.T) {
	registry := tools.NewRegistry()
	_ = registry.Register(&mockTool{name: "echo", desc: "echo"})
	orch := NewOrchestrator(nil, registry, nil, nil, nil)

	content := `I will echo that. TOOL_CALL:{"tool":"echo","args":{"msg":"ping"}}:END_TOOL_CALL`
	tc, ok := orch.parseToolCall(content)
	if !ok {
		t.Fatal("expected tool call to be parsed")
	}
	if tc.Name != "echo" {
		t.Fatalf("expected tool name echo, got %s", tc.Name)
	}
}

func TestOrchestrator_ChatWithTool(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	_ = registry.Register(&mockTool{name: "echo", desc: "echo tool", result: tools.Result{Content: "pong"}})
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: `I will echo that. TOOL_CALL:{"tool":"echo","args":{"msg":"ping"}}:END_TOOL_CALL`},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil)
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

