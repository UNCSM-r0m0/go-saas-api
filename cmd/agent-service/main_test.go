package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/runtime"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// ---- in-memory repos ----

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
func (m *memConversationRepo) ListByUser(_ context.Context, _, userID uuid.UUID, _, _ int) ([]model.Conversation, error) {
	var list []model.Conversation
	for _, conv := range m.convs {
		if conv.UserID == userID {
			list = append(list, *conv)
		}
	}
	return list, nil
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

type memArtifactRepo struct {
	arts map[uuid.UUID]*model.Artifact
}

func (m *memArtifactRepo) Create(_ context.Context, art *model.Artifact) error {
	m.arts[art.ID] = art
	return nil
}
func (m *memArtifactRepo) GetByID(_ context.Context, _, id uuid.UUID) (*model.Artifact, error) {
	return m.arts[id], nil
}
func (m *memArtifactRepo) ListByConversation(_ context.Context, _, _ uuid.UUID) ([]model.Artifact, error) {
	return nil, nil
}

var _ repository.ArtifactRepo = (*memArtifactRepo)(nil)

type memAgentRepo struct{}

func (m *memAgentRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.Agent, error) { return nil, nil }
func (m *memAgentRepo) GetDefault(_ context.Context, _ uuid.UUID) (*model.Agent, error) { return nil, nil }

var _ repository.AgentRepo = (*memAgentRepo)(nil)

// ---- mock LLM ----

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

// ---- test setup ----

func setupTestServer() *Server {
	gin.SetMode(gin.TestMode)
	log := logger.New("error")

	convRepo := &memConversationRepo{convs: make(map[uuid.UUID]*model.Conversation)}
	msgRepo := &memMessageRepo{}
	artRepo := &memArtifactRepo{arts: make(map[uuid.UUID]*model.Artifact)}
	agentRepo := &memAgentRepo{}

	llmMock := &mockLLM{
		chunks: []llm.Chunk{
			{Content: "Hello! "},
			{Content: "How can I help?"},
			{Done: true},
		},
	}

	registry := tools.NewRegistry()
	sessions := runtime.NewSessionManager(convRepo, msgRepo)
	orch := runtime.NewOrchestrator(llmMock, registry, sessions, agentRepo)

	multiClient := llm.NewMultiClient()
	return newServer(orch, convRepo, msgRepo, artRepo, log, multiClient, nil)
}

// ---- tests ----

func TestHealth(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "agent-service") {
		t.Fatal("expected response to mention agent-service")
	}
}

func TestAgentChat_MissingAuth(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agent/chat", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAgentChat_Success(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	body := `{"message":"hello"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agent/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", uuid.New().String())
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "Hello!") {
		t.Fatalf("expected SSE body to contain 'Hello!', got: %s", w.Body.String())
	}
}

func TestCreateArtifact_MissingAuth(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	body := `{"conversation_id":"` + uuid.New().String() + `","name":"test.html","type":"html","content":"<h1>hi</h1>"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/artifacts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateArtifact_Success(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	tenantID := uuid.New()
	body := `{"conversation_id":"` + uuid.New().String() + `","name":"test.html","type":"html","content":"<h1>hi</h1>"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/artifacts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var art model.Artifact
	if err := json.Unmarshal(w.Body.Bytes(), &art); err != nil {
		t.Fatalf("failed to decode artifact: %v", err)
	}
	if art.Name != "test.html" {
		t.Fatalf("expected name test.html, got %s", art.Name)
	}
	if art.TenantID != tenantID {
		t.Fatalf("expected tenant_id %s, got %s", tenantID, art.TenantID)
	}
}

func TestListConversations_MissingAuth(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/conversations", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListConversations_Success(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	tenantID := uuid.New()
	userID := uuid.New()
	convID := uuid.New()
	srv.convRepo.(*memConversationRepo).convs[convID] = &model.Conversation{
		ID:       convID,
		TenantID: tenantID,
		UserID:   userID,
		Title:    "Test Conv",
		Status:   model.ConversationActive,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/conversations", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Test Conv") {
		t.Fatalf("expected body to contain 'Test Conv', got: %s", w.Body.String())
	}
}

func TestGetConversation_Success(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	tenantID := uuid.New()
	convID := uuid.New()
	srv.convRepo.(*memConversationRepo).convs[convID] = &model.Conversation{
		ID:       convID,
		TenantID: tenantID,
		Title:    "Test Conv",
		Status:   model.ConversationActive,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/conversations/"+convID.String(), nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Test Conv") {
		t.Fatalf("expected body to contain 'Test Conv', got: %s", w.Body.String())
	}
}

func TestUpdateConversation_Success(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	tenantID := uuid.New()
	convID := uuid.New()
	srv.convRepo.(*memConversationRepo).convs[convID] = &model.Conversation{
		ID:       convID,
		TenantID: tenantID,
		Title:    "Old Title",
		Status:   model.ConversationActive,
	}

	body := `{"title":"New Title","status":"archived"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/conversations/"+convID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "New Title") {
		t.Fatalf("expected body to contain 'New Title', got: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "archived") {
		t.Fatalf("expected body to contain 'archived', got: %s", w.Body.String())
	}
}

func TestDeleteConversation_Success(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	tenantID := uuid.New()
	convID := uuid.New()
	srv.convRepo.(*memConversationRepo).convs[convID] = &model.Conversation{
		ID:       convID,
		TenantID: tenantID,
		Title:    "Test Conv",
		Status:   model.ConversationActive,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/conversations/"+convID.String(), nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListMessages_Success(t *testing.T) {
	srv := setupTestServer()
	r := srv.setupRouter()

	tenantID := uuid.New()
	convID := uuid.New()
	srv.msgRepo.(*memMessageRepo).msgs = []model.Message{
		{ID: uuid.New(), TenantID: tenantID, ConversationID: convID, Role: model.MessageRoleUser, Content: "hello"},
		{ID: uuid.New(), TenantID: tenantID, ConversationID: convID, Role: model.MessageRoleAssistant, Content: "hi there"},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/conversations/"+convID.String()+"/messages", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("expected body to contain 'hello', got: %s", w.Body.String())
	}
}

func TestPreviewArtifact_Success(t *testing.T) {
	srv := setupTestServer()
	// seed artifact
	tenantID := uuid.New()
	artID := uuid.New()
	srv.artRepo.(*memArtifactRepo).arts[artID] = &model.Artifact{
		ID:       artID,
		TenantID: tenantID,
		Content:  "preview content",
	}

	r := srv.setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/artifacts/"+artID.String()+"/preview", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "preview content" {
		t.Fatalf("expected 'preview content', got %s", w.Body.String())
	}
}
