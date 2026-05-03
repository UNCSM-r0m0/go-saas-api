package apikey

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store defines the API key repository interface.
type Store interface {
	Create(ctx context.Context, key *APIKey) error
	GetByKeyHash(ctx context.Context, keyHash string) (*APIKey, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]APIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new Postgres API key store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Create inserts a new API key.
func (s *PostgresStore) Create(ctx context.Context, key *APIKey) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO api_keys (id, user_id, name, key_hash, scopes, last_used_at, expires_at, created_at, revoked_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		key.ID, key.UserID, key.Name, key.KeyHash, key.Scopes, key.LastUsedAt, key.ExpiresAt, key.CreatedAt, key.RevokedAt)
	return err
}

// GetByKeyHash retrieves an API key by its hash (global lookup for auth).
func (s *PostgresStore) GetByKeyHash(ctx context.Context, keyHash string) (*APIKey, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, key_hash, scopes, last_used_at, expires_at, created_at, revoked_at
		 FROM api_keys WHERE key_hash = $1`, keyHash)
	return scanKey(row)
}

// ListByUser lists all non-revoked API keys for a user.
func (s *PostgresStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, key_hash, scopes, last_used_at, expires_at, created_at, revoked_at
		 FROM api_keys WHERE user_id = $1 AND revoked_at IS NULL ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []APIKey
	for rows.Next() {
		key, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *key)
	}
	return list, rows.Err()
}

// Revoke marks an API key as revoked.
func (s *PostgresStore) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = NOW() WHERE id = $1`, id)
	return err
}

// UpdateLastUsed updates the last_used_at timestamp.
func (s *PostgresStore) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func scanKey(row pgx.Row) (*APIKey, error) {
	var k APIKey
	err := row.Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.Scopes,
		&k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}
