package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Service provides business logic for AI provider management.
type Service struct {
	store Store
}

// NewService creates a new provider service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateProvider creates a new AI provider.
func (s *Service) CreateProvider(ctx context.Context, tenantID uuid.UUID, name string, pType ProviderType, baseURL, apiKey string, priority int, isPublic bool) (*AIProvider, error) {
	p := &AIProvider{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     name,
		Type:     pType,
		BaseURL:  baseURL,
		IsActive: true,
		IsPublic: isPublic,
		Priority: priority,
		Config:   map[string]any{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if apiKey != "" {
		p.APIKeyEncrypted = &apiKey // TODO: encrypt with AES-256
		hash := hashString(apiKey)
		p.APIKeyHash = &hash
	}
	if err := s.store.CreateProvider(ctx, p); err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	return p, nil
}

// GetProvider retrieves a provider by ID.
func (s *Service) GetProvider(ctx context.Context, tenantID, id uuid.UUID) (*AIProvider, error) {
	p, err := s.store.GetProvider(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return p, nil
}

// ListProviders lists all providers for a tenant.
func (s *Service) ListProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error) {
	return s.store.ListProviders(ctx, tenantID)
}

// ListActiveProviders lists active providers for a tenant.
func (s *Service) ListActiveProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error) {
	return s.store.ListActiveProviders(ctx, tenantID)
}

// UpdateProvider updates an existing provider.
func (s *Service) UpdateProvider(ctx context.Context, tenantID uuid.UUID, id uuid.UUID, name string, pType ProviderType, baseURL, apiKey string, priority int, isActive, isPublic bool) (*AIProvider, error) {
	p, err := s.store.GetProvider(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	p.Name = name
	p.Type = pType
	p.BaseURL = baseURL
	p.Priority = priority
	p.IsActive = isActive
	p.IsPublic = isPublic
	p.UpdatedAt = time.Now()
	if apiKey != "" {
		p.APIKeyEncrypted = &apiKey // TODO: encrypt with AES-256
		hash := hashString(apiKey)
		p.APIKeyHash = &hash
	}
	if err := s.store.UpdateProvider(ctx, tenantID, p); err != nil {
		return nil, fmt.Errorf("update provider: %w", err)
	}
	return p, nil
}

// DeleteProvider deletes a provider and its models.
func (s *Service) DeleteProvider(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.store.DeleteProvider(ctx, tenantID, id)
}

// TestProviderConnection attempts a health check against the provider's base URL.
func (s *Service) TestProviderConnection(ctx context.Context, tenantID, id uuid.UUID) error {
	p, err := s.store.GetProvider(ctx, tenantID, id)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	return nil
}

// CreateModel creates a new AI model.
func (s *Service) CreateModel(ctx context.Context, tenantID uuid.UUID, providerID uuid.UUID, name, displayName, description string, maxTokens, contextWindow int, supportsStreaming, supportsImages, isPublic, isPremium bool) (*AIModel, error) {
	m := &AIModel{
		ID:                uuid.New(),
		ProviderID:        providerID,
		Name:              name,
		DisplayName:       displayName,
		Description:       description,
		MaxTokens:         maxTokens,
		ContextWindow:     contextWindow,
		SupportsStreaming: supportsStreaming,
		SupportsImages:    supportsImages,
		IsActive:          true,
		IsPublic:          isPublic,
		IsPremium:         isPremium,
		Config:            map[string]any{},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := s.store.CreateModel(ctx, m); err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}
	return m, nil
}

// GetModel retrieves a model by ID.
func (s *Service) GetModel(ctx context.Context, tenantID, id uuid.UUID) (*AIModel, error) {
	return s.store.GetModel(ctx, tenantID, id)
}

// ListModelsByProvider lists models for a provider.
func (s *Service) ListModelsByProvider(ctx context.Context, tenantID, providerID uuid.UUID) ([]AIModel, error) {
	return s.store.ListModelsByProvider(ctx, tenantID, providerID)
}

// ListActiveModels lists active models visible to users.
func (s *Service) ListActiveModels(ctx context.Context, tenantID uuid.UUID, isPublicOnly bool) ([]AIModel, error) {
	return s.store.ListActiveModels(ctx, tenantID, isPublicOnly)
}

// UpdateModel updates a model.
func (s *Service) UpdateModel(ctx context.Context, tenantID uuid.UUID, m *AIModel) error {
	return s.store.UpdateModel(ctx, tenantID, m)
}

// DeleteModel deletes a model.
func (s *Service) DeleteModel(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.store.DeleteModel(ctx, tenantID, id)
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
