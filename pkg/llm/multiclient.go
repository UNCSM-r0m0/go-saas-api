package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ProviderConfig holds configuration for a single provider.
type ProviderConfig struct {
	Name    string
	Models  []string
	Client  Client
	Enabled bool
	Weight  int // Priority weight (higher = preferred)
}

// MultiClient implements Client with multi-provider support and fallback.
type MultiClient struct {
	mu        sync.RWMutex
	providers map[string]*ProviderConfig // name -> config
	modelMap  map[string]string          // model -> preferred provider name
	defaults  []string                   // fallback order by provider name
}

// NewMultiClient creates a new multi-provider client.
func NewMultiClient() *MultiClient {
	return &MultiClient{
		providers: make(map[string]*ProviderConfig),
		modelMap:  make(map[string]string),
		defaults:  []string{},
	}
}

// Register adds a provider to the manager.
func (mc *MultiClient) Register(cfg ProviderConfig) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.providers[cfg.Name] = &cfg
	for _, model := range cfg.Models {
		mc.modelMap[model] = cfg.Name
	}
	// Rebuild defaults order by weight (descending)
	mc.defaults = mc.defaults[:0]
	type weighted struct {
		name   string
		weight int
	}
	var w []weighted
	for name, p := range mc.providers {
		if p.Enabled {
			w = append(w, weighted{name, p.Weight})
		}
	}
	// Simple bubble sort by weight desc
	for i := 0; i < len(w); i++ {
		for j := i + 1; j < len(w); j++ {
			if w[i].weight < w[j].weight {
				w[i], w[j] = w[j], w[i]
			}
		}
	}
	for _, x := range w {
		mc.defaults = append(mc.defaults, x.name)
	}
}

// Stream routes the request to the appropriate provider, with fallback.
func (mc *MultiClient) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	mc.mu.RLock()
	providerNames := make([]string, 0, len(mc.defaults)+1)
	// Prefer provider mapped to model
	if preferred, ok := mc.modelMap[req.Model]; ok {
		providerNames = append(providerNames, preferred)
	}
	// Then defaults in priority order
	for _, name := range mc.defaults {
		if name != mc.modelMap[req.Model] {
			providerNames = append(providerNames, name)
		}
	}
	mc.mu.RUnlock()

	if len(providerNames) == 0 {
		return nil, fmt.Errorf("no providers registered")
	}

	var lastErr error
	for _, name := range providerNames {
		mc.mu.RLock()
		provider, ok := mc.providers[name]
		mc.mu.RUnlock()
		if !ok || !provider.Enabled || provider.Client == nil {
			continue
		}

		ch, err := provider.Client.Stream(ctx, req)
		if err != nil {
			lastErr = fmt.Errorf("provider %s: %w", name, err)
			continue
		}
		return ch, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed: %w", lastErr)
	}
	return nil, fmt.Errorf("no available providers for model %s", req.Model)
}

// HealthCheck checks all registered providers.
func (mc *MultiClient) HealthCheck(ctx context.Context) error {
	mc.mu.RLock()
	providers := make([]*ProviderConfig, 0, len(mc.providers))
	for _, p := range mc.providers {
		providers = append(providers, p)
	}
	mc.mu.RUnlock()

	var errs []error
	for _, p := range providers {
		if p.Client == nil {
			continue
		}
		ctxCheck, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := p.Client.HealthCheck(ctxCheck)
		cancel()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("health check failures: %v", errs)
	}
	return nil
}

// ListProviders returns a snapshot of registered providers.
func (mc *MultiClient) ListProviders() []ProviderConfig {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make([]ProviderConfig, 0, len(mc.providers))
	for _, p := range mc.providers {
		result = append(result, *p)
	}
	return result
}
