package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/r0lm0/go-saas-api/internal/platform/crypto"
	"github.com/r0lm0/go-saas-api/internal/provider"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// ProviderLoader loads AI providers from the database into the MultiClient.
type ProviderLoader struct {
	store       provider.Store
	multiClient *llm.MultiClient
	masterKey   string
}

// NewProviderLoader creates a new provider loader.
func NewProviderLoader(store provider.Store, multiClient *llm.MultiClient, masterKey string) *ProviderLoader {
	return &ProviderLoader{store: store, multiClient: multiClient, masterKey: masterKey}
}

// LoadAll fetches all active providers and models from the database and registers them.
func (pl *ProviderLoader) LoadAll(ctx context.Context) error {
	providers, err := pl.store.ListAllActiveProviders(ctx)
	if err != nil {
		return fmt.Errorf("list active providers: %w", err)
	}

	for _, p := range providers {
		models, err := pl.store.ListAllModelsByProvider(ctx, p.ID)
		if err != nil {
			continue // skip provider if models can't be loaded
		}

		var modelNames []string
		for _, m := range models {
			if m.IsActive {
				modelNames = append(modelNames, m.Name)
			}
		}
		if len(modelNames) == 0 {
			continue // skip provider with no active models
		}

		client, err := pl.createClient(p)
		if err != nil {
			continue // skip provider if client can't be created
		}

		// For Ollama, verify models actually exist locally before registering
		if p.Type == provider.ProviderOllama {
			modelNames = pl.filterOllamaModels(ctx, p.BaseURL, modelNames)
			if len(modelNames) == 0 {
				continue // skip Ollama provider if no models are available locally
			}
		}

		pl.multiClient.Register(llm.ProviderConfig{
			Name:    p.Name,
			Models:  modelNames,
			Client:  client,
			Enabled: true,
			Weight:  p.Priority,
		})
	}

	return nil
}

// createClient instantiates the correct LLM client based on provider type.
func (pl *ProviderLoader) createClient(p provider.AIProvider) (llm.Client, error) {
	apiKey, err := pl.decryptAPIKey(p)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key for provider %s: %w", p.Name, err)
	}

	switch p.Type {
	case provider.ProviderOllama:
		return llm.NewOllamaClient(p.BaseURL), nil
	case provider.ProviderOpenAI:
		return llm.NewOpenAIClient(apiKey), nil
	case provider.ProviderGemini:
		return llm.NewGeminiClient(apiKey), nil
	case provider.ProviderDeepSeek:
		return llm.NewDeepSeekClient(apiKey), nil
	case provider.ProviderKimi:
		return llm.NewKimiAnthropicClient(apiKey), nil
	case provider.ProviderLMStudio:
		return llm.NewLMStudioClient(p.BaseURL, apiKey), nil
	case provider.ProviderOpenCode:
		baseURL := p.BaseURL
		if baseURL == "" {
			baseURL = "https://opencode.ai/zen/go/v1"
		}
		return llm.NewOpenAICompatibleClient(baseURL, apiKey), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", p.Type)
	}
}

// decryptAPIKey decrypts the provider's API key using the master key.
func (pl *ProviderLoader) decryptAPIKey(p provider.AIProvider) (string, error) {
	if p.APIKeyEncrypted == nil || *p.APIKeyEncrypted == "" {
		return "", nil
	}
	return crypto.DecryptAPIKey(*p.APIKeyEncrypted, pl.masterKey)
}

// StartBackgroundRefresh periodically reloads providers from the database.
func (pl *ProviderLoader) StartBackgroundRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				_ = pl.LoadAll(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// filterOllamaModels fetches available models from Ollama /api/tags and returns
// only the models from dbModels that actually exist locally.
func (pl *ProviderLoader) filterOllamaModels(ctx context.Context, baseURL string, dbModels []string) []string {
	tagsURL := baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return dbModels // fallback to DB models on request error
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return dbModels // fallback to DB models on connection error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return dbModels // fallback to DB models on non-OK status
	}

	var tagResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagResp); err != nil {
		return dbModels // fallback to DB models on decode error
	}

	available := make(map[string]bool, len(tagResp.Models))
	for _, m := range tagResp.Models {
		available[m.Name] = true
	}

	var valid []string
	for _, m := range dbModels {
		if available[m] {
			valid = append(valid, m)
		}
	}
	return valid
}
