package auth

import (
	"context"

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

// Create inserts a new user
func (s *PostgresUserStore) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, email, password_hash, name, role, is_admin, messages_used_this_month, oauth_provider, oauth_subject, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := s.pool.Exec(ctx, query,
		user.ID, user.Email, user.PasswordHash,
		user.Name, user.Role, user.IsAdmin, user.MessagesUsedThisMonth,
		user.OAuthProvider, user.OAuthSubject,
		user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// GetByID retrieves a user by ID
func (s *PostgresUserStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, email, password_hash, name, role, is_admin, messages_used_this_month, oauth_provider, oauth_subject, created_at, updated_at
		FROM users WHERE id = $1
	`
	row := s.pool.QueryRow(ctx, query, id)
	return scanUser(row)
}

// GetByEmail retrieves a user by email
func (s *PostgresUserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, name, role, is_admin, messages_used_this_month, oauth_provider, oauth_subject, created_at, updated_at
		FROM users WHERE email = $1
	`
	row := s.pool.QueryRow(ctx, query, email)
	return scanUser(row)
}

// GetByOAuth retrieves a user by OAuth provider and subject
func (s *PostgresUserStore) GetByOAuth(ctx context.Context, provider, subject string) (*User, error) {
	query := `
		SELECT id, email, password_hash, name, role, is_admin, messages_used_this_month, oauth_provider, oauth_subject, created_at, updated_at
		FROM users WHERE oauth_provider = $1 AND oauth_subject = $2
	`
	row := s.pool.QueryRow(ctx, query, provider, subject)
	return scanUser(row)
}

// Update modifies an existing user
func (s *PostgresUserStore) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users SET email = $2, password_hash = $3, name = $4, role = $5,
			is_admin = $6, messages_used_this_month = $7, oauth_provider = $8, oauth_subject = $9, updated_at = $10
		WHERE id = $1
	`
	_, err := s.pool.Exec(ctx, query,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role,
		user.IsAdmin, user.MessagesUsedThisMonth,
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

// ListUsers retrieves all users with pagination
func (s *PostgresUserStore) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT id, email, password_hash, name, role, is_admin, messages_used_this_month, oauth_provider, oauth_subject, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// UpdateUserRole updates a user's role
func (s *PostgresUserStore) UpdateUserRole(ctx context.Context, id uuid.UUID, role string) error {
	query := `UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, id, role)
	return err
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.IsAdmin, &u.MessagesUsedThisMonth,
		&u.OAuthProvider, &u.OAuthSubject, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
