package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresUserStore implements UserRepository using PostgreSQL
type PostgresUserStore struct {
	pool *pgxpool.Pool
}

// NewPostgresUserStore creates a new Postgres user store
func NewPostgresUserStore(pool *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{pool: pool}
}

// setTenant sets the PostgreSQL RLS tenant context
func (s *PostgresUserStore) setTenant(ctx context.Context, tenantID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID.String())
	return err
}

// Create inserts a new user
func (s *PostgresUserStore) Create(ctx context.Context, user *User) error {
	if user.TenantID != uuid.Nil {
		if err := s.setTenant(ctx, user.TenantID); err != nil {
			return fmt.Errorf("set tenant: %w", err)
		}
	}

	query := `
		INSERT INTO users (id, tenant_id, email, password_hash, name, role, oauth_provider, oauth_subject, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := s.pool.Exec(ctx, query,
		user.ID, user.TenantID, user.Email, user.PasswordHash,
		user.Name, user.Role, user.OAuthProvider, user.OAuthSubject,
		user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// GetByID retrieves a user by ID
func (s *PostgresUserStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, name, role, oauth_provider, oauth_subject, created_at, updated_at
		FROM users WHERE id = $1
	`
	row := s.pool.QueryRow(ctx, query, id)
	return scanUser(row)
}

// GetByEmail retrieves a user by email within a tenant
func (s *PostgresUserStore) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	query := `
		SELECT id, tenant_id, email, password_hash, name, role, oauth_provider, oauth_subject, created_at, updated_at
		FROM users WHERE email = $1
	`
	row := s.pool.QueryRow(ctx, query, email)
	return scanUser(row)
}

// GetByOAuth retrieves a user by OAuth provider and subject (global lookup, bypasses RLS)
func (s *PostgresUserStore) GetByOAuth(ctx context.Context, provider, subject string) (*User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, name, role, oauth_provider, oauth_subject, created_at, updated_at
		FROM users WHERE oauth_provider = $1 AND oauth_subject = $2
	`
	row := s.pool.QueryRow(ctx, query, provider, subject)
	return scanUser(row)
}

// Update modifies an existing user
func (s *PostgresUserStore) Update(ctx context.Context, user *User) error {
	if err := s.setTenant(ctx, user.TenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	query := `
		UPDATE users SET email = $2, password_hash = $3, name = $4, role = $5,
			oauth_provider = $6, oauth_subject = $7, updated_at = $8
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role,
		user.OAuthProvider, user.OAuthSubject, user.UpdatedAt,
	)
	return err
}

// Delete removes a user
func (s *PostgresUserStore) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id)
	return err
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.OAuthProvider, &u.OAuthSubject, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
