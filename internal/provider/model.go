package provider

import (
	"strings"
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
	ProviderLMStudio  ProviderType = "lmstudio"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOpenCode  ProviderType = "opencode"
	ProviderCustom    ProviderType = "custom"
)

// AIProvider represents a configurable LLM provider in the database.
type AIProvider struct {
	ID                   uuid.UUID      `json:"id"`
	Name                 string         `json:"name"`
	Type                 ProviderType   `json:"type"`
	BaseURL              string         `json:"base_url"`
	APIKeyEncrypted      *string        `json:"-"` // never expose
	APIKeyHash           *string        `json:"-"` // never expose
	IsActive             bool           `json:"is_active"`
	IsPublic             bool           `json:"is_public"`
	Priority             int            `json:"priority"`
	Config               map[string]any `json:"config,omitempty"`
	EncryptionKeyVersion int            `json:"encryption_key_version"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// AIModel represents a model offered by a provider.
type AIModel struct {
	ID                uuid.UUID      `json:"id"`
	ProviderID        uuid.UUID      `json:"provider_id"`
	Name              string         `json:"name"`
	DisplayName       string         `json:"display_name"`
	Description       *string        `json:"description,omitempty"`
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

// SupportsWebsiteAgent returns true when this model is explicitly enabled for website generation.
func (m *AIModel) SupportsWebsiteAgent() bool {
	return configBool(m.Config, "supports_website_agent") || configBool(m.Config, "website_agent") || configNestedBool(m.Config, "capabilities", "website_agent")
}

// WebsiteAgentMaxTokens returns the model-specific output cap for website generation.
func (m *AIModel) WebsiteAgentMaxTokens() int {
	maxTokens := firstPositiveConfigInt(
		configInt(m.Config, "website_agent_max_tokens"),
		configInt(m.Config, "max_output_tokens"),
		m.MaxTokens,
		16000,
	)
	if maxTokens < 4096 {
		return 4096
	}
	if maxTokens > 64000 {
		return 64000
	}
	return maxTokens
}

func configBool(config map[string]any, key string) bool {
	if config == nil {
		return false
	}
	return anyBool(config[key])
}

func configNestedBool(config map[string]any, parent, key string) bool {
	if config == nil {
		return false
	}
	if nested, ok := config[parent].(map[string]any); ok {
		return anyBool(nested[key])
	}
	return false
}

func configInt(config map[string]any, key string) int {
	if config == nil {
		return 0
	}
	return anyInt(config[key])
}

func firstPositiveConfigInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func anyBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
	default:
		return false
	}
}

func anyInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
