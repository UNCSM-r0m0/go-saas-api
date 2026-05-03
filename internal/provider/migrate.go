package provider

import (
	"context"
	"fmt"

	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/crypto"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
)

// MigrateKeysFromEnv performs an idempotent migration:
//   1. Encrypts any existing plaintext API keys in the database.
//   2. Updates Kimi and OpenCode providers with keys from .env if the DB entry is empty.
func MigrateKeysFromEnv(ctx context.Context, store Store, masterKey string, cfg *config.Config, log logger.Logger) error {
	providers, err := store.ListAllActiveProviders(ctx)
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}

	for _, p := range providers {
		if p.APIKeyEncrypted == nil || *p.APIKeyEncrypted == "" {
			continue
		}

		// Try to decrypt. If it fails, the key is plaintext and needs encryption.
		_, err := crypto.Decrypt(*p.APIKeyEncrypted, masterKey)
		if err == nil {
			// Already encrypted, nothing to do.
			continue
		}

		// Plaintext detected — encrypt it.
		encrypted, err := crypto.Encrypt(*p.APIKeyEncrypted, masterKey)
		if err != nil {
			log.Warn("failed to encrypt plaintext API key for provider", logger.String("provider", p.Name), logger.Error(err))
			continue
		}

		p.APIKeyEncrypted = &encrypted
		if err := store.UpdateProvider(ctx, &p); err != nil {
			log.Warn("failed to update encrypted API key for provider", logger.String("provider", p.Name), logger.Error(err))
			continue
		}
		log.Info("migrated plaintext API key to encrypted", logger.String("provider", p.Name))
	}

	// Update specific providers from .env if their DB key is still empty.
	envKeys := map[string]struct {
		key     string
		baseURL string
		models  []string
	}{
		"kimi": {
			key:     cfg.KimiAPIKey,
			baseURL: cfg.KimiBaseURL,
			models:  []string{"kimi-k2-0711-preview", "moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"},
		},
		"opencode": {
			key:     cfg.OpenCodeAPIKey,
			baseURL: cfg.OpenCodeBaseURL,
			models:  []string{"glm-5.1", "glm-5", "kimi-k2.5", "kimi-k2.6", "deepseek-v4-pro", "deepseek-v4-flash", "mimo-v2-pro", "mimo-v2-omni", "mimo-v2.5-pro", "mimo-v2.5", "minimax-m2.7", "minimax-m2.5", "qwen3.6-plus", "qwen3.5-plus"},
		},
	}

	for _, p := range providers {
		spec, ok := envKeys[string(p.Type)]
		if !ok || spec.key == "" {
			continue
		}

		// Only update if the current key is empty or was plaintext (we already encrypted those above).
		if p.APIKeyEncrypted != nil && *p.APIKeyEncrypted != "" {
			// Verify it's actually encrypted
			_, err := crypto.Decrypt(*p.APIKeyEncrypted, masterKey)
			if err == nil {
				continue // Already has valid encrypted key
			}
		}

		encrypted, err := crypto.Encrypt(spec.key, masterKey)
		if err != nil {
			log.Warn("failed to encrypt env API key for provider", logger.String("provider", p.Name), logger.Error(err))
			continue
		}

		p.APIKeyEncrypted = &encrypted
		p.BaseURL = spec.baseURL
		if err := store.UpdateProvider(ctx, &p); err != nil {
			log.Warn("failed to update provider from env", logger.String("provider", p.Name), logger.Error(err))
			continue
		}
		log.Info("migrated API key from .env to database", logger.String("provider", p.Name))
	}

	return nil
}
