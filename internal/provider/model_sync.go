package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Ollama /api/tags
// ---------------------------------------------------------------------------

type ollamaTagResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string             `json:"name"`
	Model      string             `json:"model"`
	ModifiedAt time.Time          `json:"modified_at"`
	Size       int64              `json:"size"`
	Digest     string             `json:"digest"`
	Details    ollamaModelDetails `json:"details"`
}

type ollamaModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ---------------------------------------------------------------------------
// LM Studio /api/v0/models
// ---------------------------------------------------------------------------

type lmStudioModelResponse struct {
	Object string            `json:"object"`
	Data   []lmStudioModel `json:"data"`
}

type lmStudioModel struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Publisher        string `json:"publisher"`
	Arch             string `json:"arch"`
	CompatibilityType string `json:"compatibility_type"`
	Quantization     string `json:"quantization"`
	State            string `json:"state"`
	MaxContextLength int    `json:"max_context_length"`
}

// ---------------------------------------------------------------------------
// Sync result
// ---------------------------------------------------------------------------

// SyncResult contains the diff after syncing models.
type SyncResult struct {
	Added   int      `json:"added"`
	Updated int      `json:"updated"`
	Removed int      `json:"removed"`
	Errors  []string `json:"errors,omitempty"`
}

// SyncModels fetches models from a provider and syncs them with the database.
// Supports ollama (/api/tags) and lmstudio (/api/v0/models).
func (s *Service) SyncModels(ctx context.Context, tenantID, providerID uuid.UUID) (*SyncResult, error) {
	p, err := s.store.GetProvider(ctx, tenantID, providerID)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}

	switch p.Type {
	case ProviderOllama:
		return s.syncOllamaModels(ctx, tenantID, providerID, p.BaseURL)
	case ProviderLMStudio:
		return s.syncLMStudioModels(ctx, tenantID, providerID, p.BaseURL)
	default:
		return nil, fmt.Errorf("sync not supported for provider type %s", p.Type)
	}
}

func (s *Service) syncOllamaModels(ctx context.Context, tenantID, providerID uuid.UUID, baseURL string) (*SyncResult, error) {
	tagsURL := baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch ollama tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var tagResp ollamaTagResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	discovered := make(map[string]ollamaModel, len(tagResp.Models))
	for _, m := range tagResp.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		discovered[name] = m
	}

	return s.reconcileModels(ctx, tenantID, providerID, discovered)
}

func (s *Service) syncLMStudioModels(ctx context.Context, tenantID, providerID uuid.UUID, baseURL string) (*SyncResult, error) {
	modelsURL := baseURL + "/api/v0/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch lm studio models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lm studio returned status %d", resp.StatusCode)
	}

	var lmResp lmStudioModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&lmResp); err != nil {
		return nil, fmt.Errorf("decode lm studio response: %w", err)
	}

	discovered := make(map[string]lmStudioModel, len(lmResp.Data))
	for _, m := range lmResp.Data {
		discovered[m.ID] = m
	}

	return s.reconcileLMStudioModels(ctx, tenantID, providerID, discovered)
}

// reconcileModels performs the add/deactivate logic for Ollama models.
func (s *Service) reconcileModels(ctx context.Context, tenantID, providerID uuid.UUID, discovered map[string]ollamaModel) (*SyncResult, error) {
	existing, err := s.store.ListModelsByProvider(ctx, tenantID, providerID)
	if err != nil {
		return nil, fmt.Errorf("list existing models: %w", err)
	}

	existingMap := make(map[string]AIModel, len(existing))
	for _, m := range existing {
		existingMap[m.Name] = m
	}

	result := &SyncResult{}

	for name, ollamaModel := range discovered {
		if _, ok := existingMap[name]; ok {
			existingMap[name] = AIModel{} // mark as processed
			continue
		}

		displayName := name
		if ollamaModel.Details.ParameterSize != "" {
			displayName = fmt.Sprintf("%s (%s)", name, ollamaModel.Details.ParameterSize)
		}

		m := &AIModel{
			ID:                uuid.New(),
			ProviderID:        providerID,
			Name:              name,
			DisplayName:       displayName,
			Description:       fmt.Sprintf("Family: %s, Quant: %s", ollamaModel.Details.Family, ollamaModel.Details.QuantizationLevel),
			MaxTokens:         4096,
			ContextWindow:     8192,
			SupportsStreaming: true,
			SupportsImages:    false,
			IsActive:          true,
			IsPublic:          true,
			IsPremium:         false,
			Config:            map[string]any{},
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		if err := s.store.CreateModel(ctx, m); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create %s: %v", name, err))
			continue
		}
		result.Added++
	}

	for name, dbModel := range existingMap {
		if dbModel.ID == uuid.Nil {
			continue
		}
		if _, ok := discovered[name]; !ok && dbModel.IsActive {
			dbModel.IsActive = false
			dbModel.UpdatedAt = time.Now()
			if err := s.store.UpdateModel(ctx, tenantID, &dbModel); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("deactivate %s: %v", name, err))
				continue
			}
			result.Removed++
		}
	}

	return result, nil
}

// reconcileLMStudioModels performs the add/deactivate logic for LM Studio models.
func (s *Service) reconcileLMStudioModels(ctx context.Context, tenantID, providerID uuid.UUID, discovered map[string]lmStudioModel) (*SyncResult, error) {
	existing, err := s.store.ListModelsByProvider(ctx, tenantID, providerID)
	if err != nil {
		return nil, fmt.Errorf("list existing models: %w", err)
	}

	existingMap := make(map[string]AIModel, len(existing))
	for _, m := range existing {
		existingMap[m.Name] = m
	}

	result := &SyncResult{}

	for name, lmModel := range discovered {
		if _, ok := existingMap[name]; ok {
			existingMap[name] = AIModel{} // mark as processed
			continue
		}

		displayName := name
		if lmModel.Quantization != "" {
			displayName = fmt.Sprintf("%s [%s]", name, lmModel.Quantization)
		}

		ctxWindow := lmModel.MaxContextLength
		if ctxWindow == 0 {
			ctxWindow = 4096
		}

		m := &AIModel{
			ID:                uuid.New(),
			ProviderID:        providerID,
			Name:              name,
			DisplayName:       displayName,
			Description:       fmt.Sprintf("Type: %s, Arch: %s, Publisher: %s, State: %s", lmModel.Type, lmModel.Arch, lmModel.Publisher, lmModel.State),
			MaxTokens:         ctxWindow,
			ContextWindow:     ctxWindow,
			SupportsStreaming: true,
			SupportsImages:    lmModel.Type == "vlm",
			IsActive:          true,
			IsPublic:          true,
			IsPremium:         false,
			Config:            map[string]any{},
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		if err := s.store.CreateModel(ctx, m); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create %s: %v", name, err))
			continue
		}
		result.Added++
	}

	for name, dbModel := range existingMap {
		if dbModel.ID == uuid.Nil {
			continue
		}
		if _, ok := discovered[name]; !ok && dbModel.IsActive {
			dbModel.IsActive = false
			dbModel.UpdatedAt = time.Now()
			if err := s.store.UpdateModel(ctx, tenantID, &dbModel); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("deactivate %s: %v", name, err))
				continue
			}
			result.Removed++
		}
	}

	return result, nil
}
