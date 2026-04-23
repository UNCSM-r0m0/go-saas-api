package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	circuitBreakerThreshold = 3
	healthCheckTimeout      = 10 * time.Second
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
	mu             sync.RWMutex
	providers      map[string]*ProviderConfig // name -> config
	modelMap       map[string]string          // model -> preferred provider name
	defaults       []string                   // fallback order by provider name
	failureCounts  map[string]int             // consecutive failures per provider
	fallbackCount  int64                      // total fallback events
}

// NewMultiClient creates a new multi-provider client.
func NewMultiClient() *MultiClient {
	return &MultiClient{
		providers:     make(map[string]*ProviderConfig),
		modelMap:      make(map[string]string),
		defaults:      []string{},
		failureCounts: make(map[string]int),
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
	mc.rebuildDefaults()
}

func (mc *MultiClient) rebuildDefaults() {
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
	var usedProvider string
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
			mc.recordFailure(name)
			continue
		}

		usedProvider = name
		// Reset failures on success
		mc.resetFailures(name)

		// Check if fallback was used (not the preferred provider)
		if preferred, ok := mc.modelMap[req.Model]; ok && usedProvider != preferred {
			mc.mu.Lock()
			mc.fallbackCount++
			mc.mu.Unlock()
		}

		return ch, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed: %w", lastErr)
	}
	return nil, fmt.Errorf("no available providers for model %s", req.Model)
}

func (mc *MultiClient) recordFailure(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.failureCounts[name]++
	if mc.failureCounts[name] >= circuitBreakerThreshold {
		if p, ok := mc.providers[name]; ok {
			p.Enabled = false
			mc.rebuildDefaults()
		}
	}
}

func (mc *MultiClient) resetFailures(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.failureCounts[name] = 0
}

// StartHealthChecks runs periodic health checks and re-enables recovered providers.
func (mc *MultiClient) StartHealthChecks(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				mc.runHealthChecks(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (mc *MultiClient) runHealthChecks(ctx context.Context) {
	mc.mu.RLock()
	providers := make([]*ProviderConfig, 0, len(mc.providers))
	for _, p := range mc.providers {
		providers = append(providers, p)
	}
	mc.mu.RUnlock()

	for _, p := range providers {
		if p.Enabled || p.Client == nil {
			continue
		}
		ctxCheck, cancel := context.WithTimeout(ctx, healthCheckTimeout)
		err := p.Client.HealthCheck(ctxCheck)
		cancel()
		if err == nil {
			mc.mu.Lock()
			p.Enabled = true
			mc.failureCounts[p.Name] = 0
			mc.rebuildDefaults()
			mc.mu.Unlock()
		}
	}
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
		ctxCheck, cancel := context.WithTimeout(ctx, healthCheckTimeout)
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

// FallbackCount returns the number of times fallback was used.
func (mc *MultiClient) FallbackCount() int64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.fallbackCount
}
