package main

import (
	"context"
	"fmt"
	"time"

	"github.com/r0lm0/go-saas-api/internal/provider"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// ProviderLoader loads AI providers from the database into the MultiClient.
type ProviderLoader struct {
	store       provider.Store
	multiClient *llm.MultiClient
}

// NewProviderLoader creates a new provider loader.
func NewProviderLoader(store provider.Store, multiClient *llm.MultiClient) *ProviderLoader {
	return &ProviderLoader{store: store, multiClient: multiClient}
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
	switch p.Type {
	case provider.ProviderOllama:
		return llm.NewOllamaClient(p.BaseURL), nil
	case provider.ProviderOpenAI:
		apiKey := p.APIKeyEncrypted // TODO: decrypt if encrypted
		return llm.NewOpenAIClient(apiKey), nil
	case provider.ProviderGemini:
		apiKey := p.APIKeyEncrypted // TODO: decrypt if encrypted
		return llm.NewGeminiClient(apiKey), nil
	case provider.ProviderDeepSeek:
		apiKey := p.APIKeyEncrypted // TODO: decrypt if encrypted
		return llm.NewDeepSeekClient(apiKey), nil
	case provider.ProviderKimi:
		apiKey := p.APIKeyEncrypted // TODO: decrypt if encrypted
		return llm.NewKimiClient(apiKey), nil
	case provider.ProviderLMStudio:
		apiKey := p.APIKeyEncrypted // TODO: decrypt if encrypted
		return llm.NewLMStudioClient(p.BaseURL, apiKey), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", p.Type)
	}
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
