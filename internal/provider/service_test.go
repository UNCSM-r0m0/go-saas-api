package provider

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type memStore struct {
	providers map[uuid.UUID]*AIProvider
	models    map[uuid.UUID]*AIModel
}

func newMemStore() *memStore {
	return &memStore{
		providers: make(map[uuid.UUID]*AIProvider),
		models:    make(map[uuid.UUID]*AIModel),
	}
}

func (m *memStore) CreateProvider(_ context.Context, p *AIProvider) error {
	m.providers[p.ID] = p
	return nil
}

func (m *memStore) GetProvider(_ context.Context, _, id uuid.UUID) (*AIProvider, error) {
	return m.providers[id], nil
}

func (m *memStore) ListProviders(_ context.Context, _ uuid.UUID) ([]AIProvider, error) {
	var list []AIProvider
	for _, p := range m.providers {
		list = append(list, *p)
	}
	return list, nil
}

func (m *memStore) ListActiveProviders(_ context.Context, _ uuid.UUID) ([]AIProvider, error) {
	var list []AIProvider
	for _, p := range m.providers {
		if p.IsActive {
			list = append(list, *p)
		}
	}
	return list, nil
}

func (m *memStore) ListAllActiveProviders(_ context.Context) ([]AIProvider, error) {
	return m.ListActiveProviders(context.Background(), uuid.UUID{})
}

func (m *memStore) UpdateProvider(_ context.Context, _ uuid.UUID, p *AIProvider) error {
	m.providers[p.ID] = p
	return nil
}

func (m *memStore) DeleteProvider(_ context.Context, _, id uuid.UUID) error {
	delete(m.providers, id)
	return nil
}

func (m *memStore) CreateModel(_ context.Context, mod *AIModel) error {
	m.models[mod.ID] = mod
	return nil
}

func (m *memStore) GetModel(_ context.Context, _, id uuid.UUID) (*AIModel, error) {
	return m.models[id], nil
}

func (m *memStore) ListModelsByProvider(_ context.Context, _, providerID uuid.UUID) ([]AIModel, error) {
	var list []AIModel
	for _, mod := range m.models {
		if mod.ProviderID == providerID {
			list = append(list, *mod)
		}
	}
	return list, nil
}

func (m *memStore) ListAllModelsByProvider(_ context.Context, providerID uuid.UUID) ([]AIModel, error) {
	return m.ListModelsByProvider(context.Background(), uuid.UUID{}, providerID)
}

func (m *memStore) ListActiveModels(_ context.Context, _ uuid.UUID, _ bool) ([]AIModel, error) {
	var list []AIModel
	for _, mod := range m.models {
		if mod.IsActive {
			list = append(list, *mod)
		}
	}
	return list, nil
}

func (m *memStore) UpdateModel(_ context.Context, _ uuid.UUID, mod *AIModel) error {
	m.models[mod.ID] = mod
	return nil
}

func (m *memStore) DeleteModel(_ context.Context, _, id uuid.UUID) error {
	delete(m.models, id)
	return nil
}

func TestCreateProvider(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	tenantID := uuid.New()
	p, err := svc.CreateProvider(context.Background(), tenantID, "OpenAI", ProviderOpenAI, "https://api.openai.com", "sk-test", 100, true)
	if err != nil {
		t.Fatalf("create provider failed: %v", err)
	}
	if p.Name != "OpenAI" {
		t.Fatalf("expected name OpenAI, got %s", p.Name)
	}
	if p.Priority != 100 {
		t.Fatalf("expected priority 100, got %d", p.Priority)
	}
}

func TestListProviders(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	tenantID := uuid.New()
	_, _ = svc.CreateProvider(context.Background(), tenantID, "Ollama", ProviderOllama, "http://localhost:11434", "", 50, true)
	_, _ = svc.CreateProvider(context.Background(), tenantID, "OpenAI", ProviderOpenAI, "https://api.openai.com", "sk-test", 100, true)

	providers, err := svc.ListProviders(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("list providers failed: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
}

func TestUpdateProvider(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	tenantID := uuid.New()
	p, _ := svc.CreateProvider(context.Background(), tenantID, "OpenAI", ProviderOpenAI, "https://api.openai.com", "sk-test", 100, true)

	updated, err := svc.UpdateProvider(context.Background(), tenantID, p.ID, "OpenAI Updated", ProviderOpenAI, "https://api.openai.com", "sk-new", 90, false, true)
	if err != nil {
		t.Fatalf("update provider failed: %v", err)
	}
	if updated.Name != "OpenAI Updated" {
		t.Fatalf("expected updated name, got %s", updated.Name)
	}
	if updated.IsActive != false {
		t.Fatal("expected is_active to be false")
	}
}

func TestDeleteProvider(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	tenantID := uuid.New()
	p, _ := svc.CreateProvider(context.Background(), tenantID, "Gemini", ProviderGemini, "https://gemini.googleapis.com", "", 80, true)

	if err := svc.DeleteProvider(context.Background(), tenantID, p.ID); err != nil {
		t.Fatalf("delete provider failed: %v", err)
	}

	providers, _ := svc.ListProviders(context.Background(), tenantID)
	if len(providers) != 0 {
		t.Fatalf("expected 0 providers after delete, got %d", len(providers))
	}
}

func TestCreateModel(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	tenantID := uuid.New()
	providerID := uuid.New()
	m, err := svc.CreateModel(context.Background(), tenantID, providerID, "gpt-4o", "GPT-4o", "OpenAI flagship", 4096, 8192, true, false, true, false)
	if err != nil {
		t.Fatalf("create model failed: %v", err)
	}
	if m.Name != "gpt-4o" {
		t.Fatalf("expected name gpt-4o, got %s", m.Name)
	}
	if m.ProviderID != providerID {
		t.Fatal("provider_id mismatch")
	}
}

func TestListModelsByProvider(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	tenantID := uuid.New()
	providerID := uuid.New()
	_, _ = svc.CreateModel(context.Background(), tenantID, providerID, "gpt-4o", "GPT-4o", "", 4096, 8192, true, false, true, false)
	_, _ = svc.CreateModel(context.Background(), tenantID, providerID, "gpt-4o-mini", "GPT-4o Mini", "", 4096, 8192, true, false, true, false)

	models, err := svc.ListModelsByProvider(context.Background(), tenantID, providerID)
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestDeleteModel(t *testing.T) {
	store := newMemStore()
	svc := NewService(store)

	tenantID := uuid.New()
	providerID := uuid.New()
	m, _ := svc.CreateModel(context.Background(), tenantID, providerID, "gpt-3.5", "GPT-3.5", "", 4096, 4096, true, false, true, false)

	if err := svc.DeleteModel(context.Background(), tenantID, m.ID); err != nil {
		t.Fatalf("delete model failed: %v", err)
	}

	models, _ := svc.ListModelsByProvider(context.Background(), tenantID, providerID)
	if len(models) != 0 {
		t.Fatalf("expected 0 models after delete, got %d", len(models))
	}
}
