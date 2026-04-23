package fileupload

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store defines the upload repository interface.
type Store interface {
	Create(ctx context.Context, upload *Upload) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Upload, error)
	ListByUser(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]Upload, error)
	ListByConversation(ctx context.Context, tenantID, conversationID uuid.UUID) ([]Upload, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new Postgres upload store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) setTenant(ctx context.Context, tenantID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID.String())
	return err
}

// Create inserts a new upload record.
func (s *PostgresStore) Create(ctx context.Context, upload *Upload) error {
	if err := s.setTenant(ctx, upload.TenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO files (id, tenant_id, user_id, conversation_id, name, original_name, content_type, size_bytes, storage_path, metadata, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		upload.ID, upload.TenantID, upload.UserID, upload.ConversationID, upload.Name, upload.OriginalName,
		upload.ContentType, upload.SizeBytes, upload.StoragePath, upload.Metadata, upload.CreatedAt)
	return err
}

// GetByID retrieves an upload by ID.
func (s *PostgresStore) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Upload, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, conversation_id, name, original_name, content_type, size_bytes, storage_path, metadata, created_at
		 FROM files WHERE id = $1`, id)
	return scanUpload(row)
}

// ListByUser lists uploads for a user.
func (s *PostgresStore) ListByUser(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]Upload, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, conversation_id, name, original_name, content_type, size_bytes, storage_path, metadata, created_at
		 FROM files WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Upload
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *u)
	}
	return list, rows.Err()
}

// ListByConversation lists uploads attached to a conversation.
func (s *PostgresStore) ListByConversation(ctx context.Context, tenantID, conversationID uuid.UUID) ([]Upload, error) {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, conversation_id, name, original_name, content_type, size_bytes, storage_path, metadata, created_at
		 FROM files WHERE conversation_id = $1 ORDER BY created_at DESC`,
		conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Upload
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *u)
	}
	return list, rows.Err()
}

// Delete removes an upload record.
func (s *PostgresStore) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.setTenant(ctx, tenantID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM files WHERE id = $1`, id)
	return err
}

func scanUpload(row pgx.Row) (*Upload, error) {
	var u Upload
	var convID *uuid.UUID
	err := row.Scan(
		&u.ID, &u.TenantID, &u.UserID, &convID, &u.Name, &u.OriginalName,
		&u.ContentType, &u.SizeBytes, &u.StoragePath, &u.Metadata, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.ConversationID = convID
	return &u, nil
}
