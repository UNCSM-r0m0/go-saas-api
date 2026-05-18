package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
	"go.uber.org/zap"
)

// ---- mocks ----

type mockLLM struct {
	chunks  []llm.Chunk
	chunks2 []llm.Chunk
	err     error
	calls   int
}

func (m *mockLLM) Stream(_ context.Context, _ llm.Request) (<-chan llm.Chunk, error) {
	m.calls++
	var chunks []llm.Chunk
	if m.calls == 1 && m.chunks != nil {
		chunks = m.chunks
	} else if m.calls > 1 && m.chunks2 != nil {
		chunks = m.chunks2
	} else {
		chunks = m.chunks
	}
	ch := make(chan llm.Chunk)
	go func() {
		defer close(ch)
		for _, c := range chunks {
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
func (m *memConversationRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Conversation, error) {
	return m.convs[id], nil
}
func (m *memConversationRepo) ListByUser(_ context.Context, _ uuid.UUID, _, _ int) ([]model.Conversation, error) {
	return nil, nil
}
func (m *memConversationRepo) Update(_ context.Context, _ *model.Conversation) error { return nil }
func (m *memConversationRepo) Delete(_ context.Context, _ uuid.UUID) error           { return nil }

var _ repository.ConversationRepo = (*memConversationRepo)(nil)

type memMessageRepo struct {
	msgs []model.Message
}

func (m *memMessageRepo) Create(_ context.Context, msg *model.Message) error {
	m.msgs = append(m.msgs, *msg)
	return nil
}
func (m *memMessageRepo) ListByConversation(_ context.Context, _ uuid.UUID, _ int) ([]model.Message, error) {
	return m.msgs, nil
}

var _ repository.MessageRepo = (*memMessageRepo)(nil)

type memAgentRepo struct{}

func (m *memAgentRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Agent, error) { return nil, nil }
func (m *memAgentRepo) GetByRole(_ context.Context, _ model.AgentRole) (*model.Agent, error) {
	return nil, nil
}
func (m *memAgentRepo) GetDefault(_ context.Context) (*model.Agent, error) { return nil, nil }

var _ repository.AgentRepo = (*memAgentRepo)(nil)

type mockAgentRepoWithRole struct {
	agent *model.Agent
}

func (m *mockAgentRepoWithRole) GetByID(_ context.Context, _ uuid.UUID) (*model.Agent, error) {
	return nil, nil
}
func (m *mockAgentRepoWithRole) GetByRole(_ context.Context, _ model.AgentRole) (*model.Agent, error) {
	return m.agent, nil
}
func (m *mockAgentRepoWithRole) GetDefault(_ context.Context) (*model.Agent, error) { return nil, nil }

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

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()
	userID := uuid.New()

	_, ch, err := orch.Chat(ctx, userID, nil, "hi there", nil, "", "", "", "")
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

	// First LLM call returns a tool call, second call returns final text
	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "I will echo that."},
			{ToolCall: &llm.ToolCall{ID: "call_1", Name: "echo", Arguments: map[string]any{"msg": "ping"}}},
			{Done: true},
		},
		chunks2: []llm.Chunk{
			{Content: "The echo result is: pong"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	_, ch, err := orch.Chat(ctx, uuid.New(), nil, "ping", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result string
	var gotToolStart, gotToolResult bool
	for chunk := range ch {
		if chunk.Event == "tool_start" {
			gotToolStart = true
			if chunk.ToolName != "echo" {
				t.Fatalf("expected tool name echo, got %s", chunk.ToolName)
			}
		}
		if chunk.Event == "tool_result" {
			gotToolResult = true
		}
		result += chunk.Content
	}

	if !gotToolStart {
		t.Error("expected tool_start event")
	}
	if !gotToolResult {
		t.Error("expected tool_result event")
	}
	if !contains(result, "pong") {
		t.Fatalf("expected tool result pong in output, got: %q", result)
	}
	if !contains(result, "The echo result is") {
		t.Fatalf("expected second LLM response in output, got: %q", result)
	}

	// Should have: user msg, assistant msg (with tool call), tool msg, final assistant msg
	var assistantCount int
	for _, m := range msgRepo.msgs {
		if m.Role == model.MessageRoleAssistant {
			assistantCount++
		}
	}
	if assistantCount < 2 {
		t.Fatalf("expected at least 2 assistant messages, got %d", assistantCount)
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
			{Content: "I will echo that. "},
			{ToolCall: &llm.ToolCall{ID: "call_1", Name: "echo", Arguments: map[string]any{"msg": "ping"}}},
			{Done: true},
		},
		chunks2: []llm.Chunk{
			{Content: "Done!"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	_, ch, err := orch.Chat(ctx, uuid.New(), nil, "ping", nil, "", "", "", "")
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
	if !contains(result, "Done") {
		t.Fatalf("expected final response in output, got: %q", result)
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
	orch := NewOrchestrator(nil, nil, nil, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	agent, err := orch.resolveAgent(ctx, model.RoleCoder)
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
	orch := NewOrchestrator(nil, nil, nil, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	agent, err := orch.resolveAgent(ctx, model.RoleCoder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected default agent, got nil")
	}
	if agent.Role != model.RoleCoder {
		t.Fatalf("expected role coder, got %s", agent.Role)
	}
	if agent.Model != "qwen2.5-coder:3b" {
		t.Fatalf("expected default model, got %s", agent.Model)
	}
	if agent.SystemPrompt == "" {
		t.Fatal("expected non-empty system prompt for coder role")
	}
}

// ---- Phase 1 Validation Tests ----

func TestChat_UsesProviderUsage(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "Provider says hello."},
			{Done: true, Usage: &llm.Usage{PromptTokens: 100, CompletionTokens: 50}},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()
	userID := uuid.New()

	_, ch, err := orch.Chat(ctx, userID, nil, "hi", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	// Last message is assistant
	var assistantMsg *model.Message
	for i := len(msgRepo.msgs) - 1; i >= 0; i-- {
		if msgRepo.msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgRepo.msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("expected assistant message")
	}
	if assistantMsg.TokensInput != 100 {
		t.Errorf("expected TokensInput=100 (provider), got %d", assistantMsg.TokensInput)
	}
	if assistantMsg.TokensOutput != 50 {
		t.Errorf("expected TokensOutput=50 (provider), got %d", assistantMsg.TokensOutput)
	}
}

func TestChat_FallsBackToEstimationWhenProviderUsageMissing(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "No usage here."},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()
	userID := uuid.New()

	_, ch, err := orch.Chat(ctx, userID, nil, "hi", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	var assistantMsg *model.Message
	for i := len(msgRepo.msgs) - 1; i >= 0; i-- {
		if msgRepo.msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgRepo.msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("expected assistant message")
	}
	// Should be estimated (len("No usage here.") / 4 = 4)
	if assistantMsg.TokensOutput == 0 {
		t.Error("expected estimated TokensOutput > 0, got 0")
	}
}

func TestChat_MultiToolLoop_AccumulatesUsage(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	_ = registry.Register(&mockTool{name: "echo", desc: "echo tool", result: tools.Result{Content: "pong"}})
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "Using tool..."},
			{ToolCall: &llm.ToolCall{ID: "call_1", Name: "echo", Arguments: map[string]any{"msg": "ping"}}},
			{Done: true, Usage: &llm.Usage{PromptTokens: 50, CompletionTokens: 20}},
		},
		chunks2: []llm.Chunk{
			{Content: "Done!"},
			{Done: true, Usage: &llm.Usage{PromptTokens: 80, CompletionTokens: 30}},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()
	userID := uuid.New()

	_, ch, err := orch.Chat(ctx, userID, nil, "ping", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	var assistantMsg *model.Message
	for i := len(msgRepo.msgs) - 1; i >= 0; i-- {
		if msgRepo.msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgRepo.msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("expected assistant message")
	}
	// 50 + 80 = 130 input, 20 + 30 = 50 output
	if assistantMsg.TokensInput != 130 {
		t.Errorf("expected accumulated TokensInput=130, got %d", assistantMsg.TokensInput)
	}
	if assistantMsg.TokensOutput != 50 {
		t.Errorf("expected accumulated TokensOutput=50, got %d", assistantMsg.TokensOutput)
	}
}

func TestChat_ConversationOwnership_WrongUser(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	userA := uuid.New()
	userB := uuid.New()
	convID := uuid.New()
	convRepo.convs[convID] = &model.Conversation{
		ID:     convID,
		UserID: userA,
	}

	llmMock := &mockLLM{}
	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	_, _, err := orch.Chat(ctx, userB, &convID, "hi", nil, "", "", "", "")
	if err == nil {
		t.Fatal("expected error for conversation ownership violation")
	}
	if !contains(err.Error(), "does not belong to user") {
		t.Fatalf("expected ownership error, got: %v", err)
	}
}

func TestChat_ConversationOwnership_CorrectUser(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	userA := uuid.New()
	convID := uuid.New()
	convRepo.convs[convID] = &model.Conversation{
		ID:     convID,
		UserID: userA,
	}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "Hello!"},
			{Done: true},
		},
	}
	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	gotConvID, ch, err := orch.Chat(ctx, userA, &convID, "hi", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotConvID != convID {
		t.Fatalf("expected convID %s, got %s", convID, gotConvID)
	}
	for range ch {
	}
}


func TestChat_FlowContinuity_FromMetadata(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	userID := uuid.New()
	convID := uuid.New()
	convRepo.convs[convID] = &model.Conversation{
		ID:     convID,
		UserID: userID,
		Metadata: map[string]any{
			"flow":             "api_integration",
			"status":           "collecting",
			"collected_fields": map[string]string{"target_platform": "WooCommerce"},
		},
	}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "What is the external API name?"},
			{Content: "\n\n---FLOW_STATE---\n{\"extracted_fields\":{},\"step\":\"collecting\"}"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	// Message that would NOT trigger api_integration detection on its own
	_, ch, err := orch.Chat(ctx, userID, &convID, "ok", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var streamed string
	for chunk := range ch {
		streamed += chunk.Content
	}

	// 1. Streamed content must NOT contain FLOW_STATE
	if strings.Contains(streamed, "---FLOW_STATE---") {
		t.Errorf("streamed content should not contain FLOW_STATE block, got: %q", streamed)
	}

	// 2. Saved assistant message must NOT contain FLOW_STATE
	var assistantMsg *model.Message
	for i := range msgRepo.msgs {
		if msgRepo.msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgRepo.msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("expected assistant message to be saved")
	}
	if strings.Contains(assistantMsg.Content, "---FLOW_STATE---") {
		t.Errorf("saved assistant message should not contain FLOW_STATE block, got: %q", assistantMsg.Content)
	}

	// 3. Metadata should still have api_integration flow
	updatedConv := convRepo.convs[convID]
	if updatedConv == nil {
		t.Fatal("conversation not found after chat")
	}
	if updatedConv.Metadata["flow"] != "api_integration" {
		t.Fatalf("expected flow continuity, got %v", updatedConv.Metadata["flow"])
	}
}

func TestChat_FlowState_SplitAcrossChunks(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	userID := uuid.New()
	convID := uuid.New()
	convRepo.convs[convID] = &model.Conversation{
		ID:     convID,
		UserID: userID,
		Metadata: map[string]any{
			"flow":             "api_integration",
			"status":           "collecting",
			"collected_fields": map[string]string{"target_platform": "WooCommerce"},
		},
	}

	// Split the FLOW_STATE marker across chunks
	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "Pregunta visible "},
			{Content: "---FLOW_"},
			{Content: "STATE---\n{\"extracted_fields\":{},\"step\":\"collecting\"}"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	_, ch, err := orch.Chat(ctx, userID, &convID, "ok", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var streamed string
	for chunk := range ch {
		streamed += chunk.Content
	}

	// Stream must not contain any part of the marker
	if strings.Contains(streamed, "---FLOW_") {
		t.Errorf("streamed content leaked marker prefix: %q", streamed)
	}
	if strings.Contains(streamed, "FLOW_STATE") {
		t.Errorf("streamed content leaked marker text: %q", streamed)
	}

	// Saved assistant message must also not contain the marker
	var assistantMsg *model.Message
	for i := range msgRepo.msgs {
		if msgRepo.msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgRepo.msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("expected assistant message")
	}
	if strings.Contains(assistantMsg.Content, "---FLOW_") {
		t.Errorf("saved message leaked marker prefix: %q", assistantMsg.Content)
	}
	if strings.Contains(assistantMsg.Content, "FLOW_STATE") {
		t.Errorf("saved message leaked marker text: %q", assistantMsg.Content)
	}
}

func TestChat_EnforceSingleQuestion_Applied(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	userID := uuid.New()
	convID := uuid.New()
	convRepo.convs[convID] = &model.Conversation{
		ID:     convID,
		UserID: userID,
		Metadata: map[string]any{
			"flow":             "api_integration",
			"status":           "collecting",
			"collected_fields": map[string]string{"target_platform": "WooCommerce"},
		},
	}

	// LLM returns 2 questions (should be sanitized to 1)
	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "What is the external API name? Also, what is the integration goal?"},
			{Content: "\n\n---FLOW_STATE---\n{\"extracted_fields\":{},\"step\":\"collecting\"}"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	_, ch, err := orch.Chat(ctx, userID, &convID, "ok", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	var assistantMsg *model.Message
	for i := range msgRepo.msgs {
		if msgRepo.msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgRepo.msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("expected assistant message")
	}
	if strings.Count(assistantMsg.Content, "?") > 1 {
		t.Errorf("stored content has %d questions, expected at most 1: %q", strings.Count(assistantMsg.Content, "?"), assistantMsg.Content)
	}
}


func TestChat_FileWrite_SetsGeneratedStatus(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	_ = registry.Register(&mockTool{name: "file_write", desc: "write file", result: tools.Result{Content: "File written successfully"}})
	agentRepo := &memAgentRepo{}

	userID := uuid.New()
	convID := uuid.New()
	convRepo.convs[convID] = &model.Conversation{
		ID:     convID,
		UserID: userID,
		Metadata: map[string]any{
			"flow":   "api_integration",
			"status": "collecting",
			"collected_fields": map[string]string{
				"target_platform":    "WooCommerce",
				"external_api_name":  "Stripe",
				"integration_goal":   "sync payments",
				"data_direction":     "import",
				"auth_type":          "api_key",
				"data_entities":      "orders",
				"frequency":          "real_time",
				"deliverable_type":   "plugin",
			},
		},
	}

	// First LLM call emits file_write, second call emits final text
	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "I'll write the scope document now."},
			{ToolCall: &llm.ToolCall{ID: "call_1", Name: "file_write", Arguments: map[string]any{"name": "01-project-scope.md", "content": "# Scope"}}},
			{Done: true},
		},
		chunks2: []llm.Chunk{
			{Content: "Scope document created successfully."},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	_, ch, err := orch.Chat(ctx, userID, &convID, "generate scope", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	updatedConv := convRepo.convs[convID]
	if updatedConv == nil {
		t.Fatal("conversation not found after chat")
	}
	if updatedConv.Metadata["status"] != "generated" {
		t.Fatalf("expected status 'generated', got %v", updatedConv.Metadata["status"])
	}

	deliverables, ok := updatedConv.Metadata["deliverables"].([]string)
	if !ok {
		// May be stored as []interface{} depending on JSON round-trip; check via reflection in real DB.
		// In-memory it stays []string.
		t.Fatalf("expected deliverables to be []string, got %T", updatedConv.Metadata["deliverables"])
	}
	found := false
	for _, d := range deliverables {
		if d == "01-project-scope.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected deliverables to contain '01-project-scope.md', got %v", deliverables)
	}
}

func TestChat_EnforceSingleQuestion_StreamAndStore(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	userID := uuid.New()
	convID := uuid.New()
	convRepo.convs[convID] = &model.Conversation{
		ID:     convID,
		UserID: userID,
		Metadata: map[string]any{
			"flow":             "api_integration",
			"status":           "collecting",
			"collected_fields": map[string]string{"target_platform": "WooCommerce"},
		},
	}

	// LLM returns 2 questions (should be sanitized to 1 in both stream and store)
	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "What is the external API name? Also, what is the integration goal?"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()

	_, ch, err := orch.Chat(ctx, userID, &convID, "ok", nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var streamed string
	for chunk := range ch {
		streamed += chunk.Content
	}

	if strings.Count(streamed, "?") > 1 {
		t.Errorf("streamed content has %d questions, expected at most 1: %q", strings.Count(streamed, "?"), streamed)
	}

	var assistantMsg *model.Message
	for i := range msgRepo.msgs {
		if msgRepo.msgs[i].Role == model.MessageRoleAssistant {
			assistantMsg = &msgRepo.msgs[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("expected assistant message")
	}
	if strings.Count(assistantMsg.Content, "?") > 1 {
		t.Errorf("stored content has %d questions, expected at most 1: %q", strings.Count(assistantMsg.Content, "?"), assistantMsg.Content)
	}
}


// ---- Temporal Short-Circuit Tests ----

func TestOrchestrator_Chat_TemporalShortCircuit(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "should not be used"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()
	userID := uuid.New()

	convID, ch, err := orch.Chat(ctx, userID, nil, "qué día es hoy", nil, "", "", "", "America/Guatemala")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if convID == uuid.Nil {
		t.Fatal("expected valid conversation ID")
	}

	var streamed string
	for chunk := range ch {
		streamed += chunk.Content
	}

	if !strings.Contains(streamed, "Hoy es") {
		t.Errorf("expected temporal response to contain 'Hoy es', got: %q", streamed)
	}

	// LLM must NOT have been called
	if llmMock.calls != 0 {
		t.Errorf("expected LLM mock to not be called, got %d calls", llmMock.calls)
	}

	// Assistant message must have been persisted
	var foundAssistant bool
	for _, m := range msgRepo.msgs {
		if m.Role == model.MessageRoleAssistant && strings.Contains(m.Content, "Hoy es") {
			foundAssistant = true
			break
		}
	}
	if !foundAssistant {
		t.Error("expected assistant temporal message to be persisted")
	}
}

func TestOrchestrator_Chat_TemporalInvalidTimezone(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{chunks: []llm.Chunk{{Content: "unused"}, {Done: true}}}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()
	userID := uuid.New()

	_, ch, err := orch.Chat(ctx, userID, nil, "qué fecha es hoy", nil, "", "", "", "invalid-zone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var streamed string
	for chunk := range ch {
		streamed += chunk.Content
	}

	if !strings.Contains(streamed, "Hoy es") {
		t.Errorf("expected fallback response, got: %q", streamed)
	}
	if llmMock.calls != 0 {
		t.Errorf("expected LLM mock to not be called, got %d calls", llmMock.calls)
	}
}

func TestOrchestrator_Chat_NonTemporalStillUsesLLM(t *testing.T) {
	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	sessions := NewSessionManager(convRepo, msgRepo)
	registry := tools.NewRegistry()
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "Hello!"},
			{Done: true},
		},
	}

	orch := NewOrchestrator(llmMock, registry, sessions, agentRepo, nil, nil, nil, nil, nil, nil, nil, logger.Logger{Logger: zap.NewNop()})
	ctx := context.Background()
	userID := uuid.New()

	_, ch, err := orch.Chat(ctx, userID, nil, "hola, cómo estás", nil, "", "", "", "America/Guatemala")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var streamed string
	for chunk := range ch {
		streamed += chunk.Content
	}

	if streamed != "Hello!" {
		t.Errorf("expected LLM response, got: %q", streamed)
	}
	if llmMock.calls == 0 {
		t.Error("expected LLM mock to be called for non-temporal query")
	}
}
