package provider

import (
	"time"

	"github.com/google/uuid"
)

// ProviderType represents the LLM provider type.
type ProviderType string

const (
	ProviderOllama    ProviderType = "ollama"
	ProviderOpenAI    ProviderType = "openai"
	ProviderGemini    ProviderType = "gemini"
	ProviderDeepSeek  ProviderType = "deepseek"
	ProviderKimi      ProviderType = "kimi"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderCustom    ProviderType = "custom"
)

// AIProvider represents a configurable LLM provider in the database.
type AIProvider struct {
	ID              uuid.UUID      `json:"id"`
	TenantID        uuid.UUID      `json:"tenant_id"`
	Name            string         `json:"name"`
	Type            ProviderType   `json:"type"`
	BaseURL         string         `json:"base_url"`
	APIKeyEncrypted string         `json:"-"` // never expose
	APIKeyHash      string         `json:"-"` // never expose
	IsActive        bool           `json:"is_active"`
	IsPublic        bool           `json:"is_public"`
	Priority        int            `json:"priority"`
	Config          map[string]any `json:"config,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// AIModel represents a model offered by a provider.
type AIModel struct {
	ID                uuid.UUID      `json:"id"`
	ProviderID        uuid.UUID      `json:"provider_id"`
	Name              string         `json:"name"`
	DisplayName       string         `json:"display_name"`
	Description       string         `json:"description,omitempty"`
	MaxTokens         int            `json:"max_tokens"`
	ContextWindow     int            `json:"context_window"`
	SupportsStreaming bool           `json:"supports_streaming"`
	SupportsImages    bool           `json:"supports_images"`
	IsActive          bool           `json:"is_active"`
	IsPublic          bool           `json:"is_public"`
	IsPremium         bool           `json:"is_premium"`
	Config            map[string]any `json:"config,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// IsEnabled returns true if the provider is active.
func (p *AIProvider) IsEnabled() bool {
	return p.IsActive
}

// IsEnabled returns true if the model is active.
func (m *AIModel) IsEnabled() bool {
	return m.IsActive
}
