package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStoreForSync implements just enough of Store for sync tests
type mockStoreForSync struct {
	provider  *AIProvider
	models    []AIModel
	created   []AIModel
	updated   []AIModel
}

func (m *mockStoreForSync) CreateProvider(ctx context.Context, p *AIProvider) error { return nil }
func (m *mockStoreForSync) GetProvider(ctx context.Context, tenantID, id uuid.UUID) (*AIProvider, error) {
	if m.provider == nil {
		return nil, fmt.Errorf("not found")
	}
	return m.provider, nil
}
func (m *mockStoreForSync) ListProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error) { return nil, nil }
func (m *mockStoreForSync) ListActiveProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error) { return nil, nil }
func (m *mockStoreForSync) ListAllActiveProviders(ctx context.Context) ([]AIProvider, error) { return nil, nil }
func (m *mockStoreForSync) UpdateProvider(ctx context.Context, tenantID uuid.UUID, p *AIProvider) error { return nil }
func (m *mockStoreForSync) DeleteProvider(ctx context.Context, tenantID, id uuid.UUID) error { return nil }

func (m *mockStoreForSync) CreateModel(ctx context.Context, model *AIModel) error {
	m.created = append(m.created, *model)
	return nil
}
func (m *mockStoreForSync) GetModel(ctx context.Context, tenantID, id uuid.UUID) (*AIModel, error) { return nil, nil }
func (m *mockStoreForSync) ListModelsByProvider(ctx context.Context, tenantID, providerID uuid.UUID) ([]AIModel, error) {
	return m.models, nil
}
func (m *mockStoreForSync) ListAllModelsByProvider(ctx context.Context, providerID uuid.UUID) ([]AIModel, error) { return nil, nil }
func (m *mockStoreForSync) ListActiveModels(ctx context.Context, tenantID uuid.UUID, isPublicOnly bool) ([]AIModel, error) { return nil, nil }
func (m *mockStoreForSync) UpdateModel(ctx context.Context, tenantID uuid.UUID, model *AIModel) error {
	m.updated = append(m.updated, *model)
	return nil
}
func (m *mockStoreForSync) DeleteModel(ctx context.Context, tenantID, id uuid.UUID) error { return nil }

func TestSyncModels_Ollama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"models":[{"name":"qwen2.5:7b","model":"qwen2.5:7b","details":{"parameter_size":"7B","family":"qwen2"}}]}`)
	}))
	defer server.Close()

	store := &mockStoreForSync{
		provider: &AIProvider{
			ID:      uuid.New(),
			TenantID: uuid.New(),
			Name:    "Ollama Test",
			Type:    ProviderOllama,
			BaseURL: server.URL,
		},
		models: []AIModel{},
	}

	svc := NewService(store, "test-master-key-32-bytes-long!!")
	result, err := svc.SyncModels(context.Background(), store.provider.TenantID, store.provider.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Added)
	assert.Equal(t, 0, result.Removed)
	assert.Len(t, store.created, 1)
	assert.Equal(t, "qwen2.5:7b", store.created[0].Name)
}

func TestSyncModels_LMStudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v0/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"object":"list","data":[{"id":"qwen2-vl-7b","type":"vlm","quantization":"4bit","state":"loaded","max_context_length":32768},{"id":"llama-3.1-8b","type":"llm","quantization":"Q4_K_M","state":"not-loaded","max_context_length":131072}]}`)
	}))
	defer server.Close()

	store := &mockStoreForSync{
		provider: &AIProvider{
			ID:      uuid.New(),
			TenantID: uuid.New(),
			Name:    "LM Studio Test",
			Type:    ProviderLMStudio,
			BaseURL: server.URL,
		},
		models: []AIModel{},
	}

	svc := NewService(store, "test-master-key-32-bytes-long!!")
	result, err := svc.SyncModels(context.Background(), store.provider.TenantID, store.provider.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Added)
	assert.Equal(t, 0, result.Removed)
	require.Len(t, store.created, 2)

	// Build map since order is non-deterministic (Go map iteration)
	createdMap := make(map[string]AIModel)
	for _, m := range store.created {
		createdMap[m.Name] = m
	}

	qwen, ok := createdMap["qwen2-vl-7b"]
	require.True(t, ok, "qwen2-vl-7b should be created")
	assert.True(t, qwen.SupportsImages) // vlm type
	assert.Equal(t, 32768, qwen.ContextWindow)

	llama, ok := createdMap["llama-3.1-8b"]
	require.True(t, ok, "llama-3.1-8b should be created")
	assert.False(t, llama.SupportsImages)
	assert.Equal(t, 131072, llama.ContextWindow)
}

func TestSyncModels_DeactivatesMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer server.Close()

	existingID := uuid.New()
	store := &mockStoreForSync{
		provider: &AIProvider{
			ID:      uuid.New(),
			TenantID: uuid.New(),
			Name:    "Ollama Test",
			Type:    ProviderOllama,
			BaseURL: server.URL,
		},
		models: []AIModel{
			{ID: existingID, Name: "old-model", DisplayName: "Old Model", IsActive: true, ProviderID: uuid.New()},
		},
	}

	svc := NewService(store, "test-master-key-32-bytes-long!!")
	result, err := svc.SyncModels(context.Background(), store.provider.TenantID, store.provider.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Added)
	assert.Equal(t, 1, result.Removed)
	require.Len(t, store.updated, 1)
	assert.False(t, store.updated[0].IsActive)
}
