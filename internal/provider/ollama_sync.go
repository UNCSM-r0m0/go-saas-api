package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// OllamaTagResponse represents the response from Ollama's /api/tags endpoint.
type OllamaTagResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaModel represents a single model in the Ollama tags response.
type OllamaModel struct {
	Name       string            `json:"name"`
	Model      string            `json:"model"`
	ModifiedAt time.Time         `json:"modified_at"`
	Size       int64             `json:"size"`
	Digest     string            `json:"digest"`
	Details    OllamaModelDetails `json:"details"`
}

// OllamaModelDetails contains metadata about an Ollama model.
type OllamaModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// SyncResult contains the diff after syncing Ollama models.
type SyncResult struct {
	Added   int      `json:"added"`
	Updated int      `json:"updated"`
	Removed int      `json:"removed"`
	Errors  []string `json:"errors,omitempty"`
}

// SyncOllamaModels fetches models from an Ollama provider and syncs them with the database.
func (s *Service) SyncOllamaModels(ctx context.Context, tenantID, providerID uuid.UUID) (*SyncResult, error) {
	p, err := s.store.GetProvider(ctx, tenantID, providerID)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	if p.Type != ProviderOllama {
		return nil, fmt.Errorf("provider type must be ollama, got %s", p.Type)
	}

	// Fetch models from Ollama
	tagsURL := p.BaseURL + "/api/tags"
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

	var tagResp OllamaTagResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	// Build a set of discovered model names
	discovered := make(map[string]OllamaModel, len(tagResp.Models))
	for _, m := range tagResp.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		discovered[name] = m
	}

	// Fetch existing models from DB
	existing, err := s.store.ListModelsByProvider(ctx, tenantID, providerID)
	if err != nil {
		return nil, fmt.Errorf("list existing models: %w", err)
	}

	existingMap := make(map[string]AIModel, len(existing))
	for _, m := range existing {
		existingMap[m.Name] = m
	}

	result := &SyncResult{}

	// Add or update models
	for name, ollamaModel := range discovered {
		if _, ok := existingMap[name]; ok {
			// Model exists â€” optionally update metadata
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

	// Deactivate models that no longer exist in Ollama
	for name, dbModel := range existingMap {
		if dbModel.ID == uuid.Nil {
			continue // was processed above
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
