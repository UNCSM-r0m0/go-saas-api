package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store defines the AI provider repository interface.
type Store interface {
	// Providers
	CreateProvider(ctx context.Context, p *AIProvider) error
	GetProvider(ctx context.Context, tenantID, id uuid.UUID) (*AIProvider, error)
	ListProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error)
	ListActiveProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error)
	ListAllActiveProviders(ctx context.Context) ([]AIProvider, error) // bypass RLS for loader
	UpdateProvider(ctx context.Context, tenantID uuid.UUID, p *AIProvider) error
	DeleteProvider(ctx context.Context, tenantID, id uuid.UUID) error

	// Models
	CreateModel(ctx context.Context, m *AIModel) error
	GetModel(ctx context.Context, tenantID, id uuid.UUID) (*AIModel, error)
	ListModelsByProvider(ctx context.Context, tenantID, providerID uuid.UUID) ([]AIModel, error)
	ListAllModelsByProvider(ctx context.Context, providerID uuid.UUID) ([]AIModel, error) // bypass RLS for loader
	ListActiveModels(ctx context.Context, tenantID uuid.UUID, isPublicOnly bool) ([]AIModel, error)
	UpdateModel(ctx context.Context, tenantID uuid.UUID, m *AIModel) error
	DeleteModel(ctx context.Context, tenantID, id uuid.UUID) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new Postgres provider store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) setTenant(ctx context.Context, tenantID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID.String())
	return err
}

// CreateProvider inserts a new AI provider.
func (s *PostgresStore) CreateProvider(ctx context.Context, p *AIProvider) error {
	if err := s.setTenant(ctx, p.TenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO ai_providers (id, tenant_id, name, type, base_url, api_key_encrypted, api_key_hash, is_active, is_public, priority, config, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		p.ID, p.TenantID, p.Name, p.Type, p.BaseURL, p.APIKeyEncrypted, p.APIKeyHash, p.IsActive, p.IsPublic, p.Priority, p.Config, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetProvider retrieves a provider by ID.
func (s *PostgresStore) GetProvider(ctx context.Context, tenantID, id uuid.UUID) (*AIProvider, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, type, base_url, api_key_encrypted, api_key_hash, is_active, is_public, priority, config, created_at, updated_at
		 FROM ai_providers WHERE id = $1`, id)
	return scanProvider(row)
}

// ListProviders lists all providers for a tenant.
func (s *PostgresStore) ListProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, type, base_url, api_key_encrypted, api_key_hash, is_active, is_public, priority, config, created_at, updated_at
		 FROM ai_providers ORDER BY priority DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProviders(rows)
}

// ListActiveProviders lists active providers for a tenant.
func (s *PostgresStore) ListActiveProviders(ctx context.Context, tenantID uuid.UUID) ([]AIProvider, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, type, base_url, api_key_encrypted, api_key_hash, is_active, is_public, priority, config, created_at, updated_at
		 FROM ai_providers WHERE is_active = true ORDER BY priority DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProviders(rows)
}

// ListAllActiveProviders lists all active providers bypassing RLS (for service loader).
func (s *PostgresStore) ListAllActiveProviders(ctx context.Context) ([]AIProvider, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, type, base_url, api_key_encrypted, api_key_hash, is_active, is_public, priority, config, created_at, updated_at
		 FROM ai_providers WHERE is_active = true ORDER BY priority DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProviders(rows)
}

// UpdateProvider updates a provider.
func (s *PostgresStore) UpdateProvider(ctx context.Context, tenantID uuid.UUID, p *AIProvider) error {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE ai_providers
		 SET name = $1, type = $2, base_url = $3, api_key_encrypted = $4, api_key_hash = $5,
		     is_active = $6, is_public = $7, priority = $8, config = $9, updated_at = NOW()
		 WHERE id = $10`,
		p.Name, p.Type, p.BaseURL, p.APIKeyEncrypted, p.APIKeyHash, p.IsActive, p.IsPublic, p.Priority, p.Config, p.ID)
	return err
}

// DeleteProvider deletes a provider (cascades to models).
func (s *PostgresStore) DeleteProvider(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM ai_providers WHERE id = $1`, id)
	return err
}

// CreateModel inserts a new AI model.
func (s *PostgresStore) CreateModel(ctx context.Context, m *AIModel) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO ai_models (id, provider_id, name, display_name, description, max_tokens, context_window, supports_streaming, supports_images, is_active, is_public, is_premium, config, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		m.ID, m.ProviderID, m.Name, m.DisplayName, m.Description, m.MaxTokens, m.ContextWindow, m.SupportsStreaming, m.SupportsImages, m.IsActive, m.IsPublic, m.IsPremium, m.Config, m.CreatedAt, m.UpdatedAt)
	return err
}

// GetModel retrieves a model by ID.
func (s *PostgresStore) GetModel(ctx context.Context, tenantID, id uuid.UUID) (*AIModel, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	row := s.pool.QueryRow(ctx,
		`SELECT id, provider_id, name, display_name, description, max_tokens, context_window, supports_streaming, supports_images, is_active, is_public, is_premium, config, created_at, updated_at
		 FROM ai_models WHERE id = $1`, id)
	return scanModel(row)
}

// ListModelsByProvider lists models for a specific provider.
func (s *PostgresStore) ListModelsByProvider(ctx context.Context, tenantID, providerID uuid.UUID) ([]AIModel, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, provider_id, name, display_name, description, max_tokens, context_window, supports_streaming, supports_images, is_active, is_public, is_premium, config, created_at, updated_at
		 FROM ai_models WHERE provider_id = $1 ORDER BY name`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

// ListAllModelsByProvider lists models for a provider bypassing RLS (for service loader).
func (s *PostgresStore) ListAllModelsByProvider(ctx context.Context, providerID uuid.UUID) ([]AIModel, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, provider_id, name, display_name, description, max_tokens, context_window, supports_streaming, supports_images, is_active, is_public, is_premium, config, created_at, updated_at
		 FROM ai_models WHERE provider_id = $1 AND is_active = true ORDER BY name`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

// ListActiveModels lists active models for a tenant.
func (s *PostgresStore) ListActiveModels(ctx context.Context, tenantID uuid.UUID, isPublicOnly bool) ([]AIModel, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	q := `SELECT m.id, m.provider_id, m.name, m.display_name, m.description, m.max_tokens, m.context_window, m.supports_streaming, m.supports_images, m.is_active, m.is_public, m.is_premium, m.config, m.created_at, m.updated_at
		  FROM ai_models m
		  JOIN ai_providers p ON m.provider_id = p.id
		  WHERE m.is_active = true AND p.is_active = true AND p.tenant_id = current_tenant()`
	if isPublicOnly {
		q += ` AND m.is_public = true`
	}
	q += ` ORDER BY m.name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

// UpdateModel updates a model.
func (s *PostgresStore) UpdateModel(ctx context.Context, tenantID uuid.UUID, m *AIModel) error {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE ai_models
		 SET name = $1, display_name = $2, description = $3, max_tokens = $4, context_window = $5,
		     supports_streaming = $6, supports_images = $7, is_active = $8, is_public = $9, is_premium = $10, config = $11, updated_at = NOW()
		 WHERE id = $12`,
		m.Name, m.DisplayName, m.Description, m.MaxTokens, m.ContextWindow, m.SupportsStreaming, m.SupportsImages, m.IsActive, m.IsPublic, m.IsPremium, m.Config, m.ID)
	return err
}

// DeleteModel deletes a model.
func (s *PostgresStore) DeleteModel(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM ai_models WHERE id = $1`, id)
	return err
}

func scanProvider(row pgx.Row) (*AIProvider, error) {
	var p AIProvider
	var configBytes []byte
	err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Type, &p.BaseURL, &p.APIKeyEncrypted, &p.APIKeyHash,
		&p.IsActive, &p.IsPublic, &p.Priority, &configBytes, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(configBytes) > 0 {
		_ = json.Unmarshal(configBytes, &p.Config)
	}
	return &p, nil
}

func scanProviders(rows pgx.Rows) ([]AIProvider, error) {
	var list []AIProvider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

func scanModel(row pgx.Row) (*AIModel, error) {
	var m AIModel
	var configBytes []byte
	err := row.Scan(
		&m.ID, &m.ProviderID, &m.Name, &m.DisplayName, &m.Description, &m.MaxTokens, &m.ContextWindow,
		&m.SupportsStreaming, &m.SupportsImages, &m.IsActive, &m.IsPublic, &m.IsPremium, &configBytes, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(configBytes) > 0 {
		_ = json.Unmarshal(configBytes, &m.Config)
	}
	return &m, nil
}

func scanModels(rows pgx.Rows) ([]AIModel, error) {
	var list []AIModel
	for rows.Next() {
		m, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *m)
	}
	return list, rows.Err()
}
