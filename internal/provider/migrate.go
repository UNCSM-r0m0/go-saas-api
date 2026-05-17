package provider

import (
	"context"
	"fmt"

	"github.com/r0lm0/go-saas-api/internal/platform/crypto"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
)

// MigrateKeys encrypts any plaintext API keys found in the database.
// Provider configuration is managed exclusively through the admin API now.
func MigrateKeys(ctx context.Context, store Store, masterKey string, log logger.Logger) error {
	providers, err := store.ListAllActiveProviders(ctx)
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}

	for _, p := range providers {
		if p.APIKeyEncrypted == nil || *p.APIKeyEncrypted == "" {
			continue
		}

		_, err := crypto.Decrypt(*p.APIKeyEncrypted, masterKey)
		if err == nil {
			continue
		}

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

	return nil
}