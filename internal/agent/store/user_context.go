package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
)

// UserContextStore handles persistent user context (cross-session memory).
type UserContextStore struct {
	pool *pgxpool.Pool
}

// NewUserContextStore creates a new user context store.
func NewUserContextStore(pool *pgxpool.Pool) *UserContextStore {
	return &UserContextStore{pool: pool}
}

var _ repository.UserContextRepo = (*UserContextStore)(nil)

// GetByUser retrieves all context items for a user.
func (s *UserContextStore) GetByUser(ctx context.Context, userID uuid.UUID) ([]model.UserContextItem, error) {
	if s.pool == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT user_id, key, value, source, confidence, updated_at
		 FROM user_context WHERE user_id = $1 ORDER BY updated_at DESC`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("query user_context: %w", err)
	}
	defer rows.Close()

	var items []model.UserContextItem
	for rows.Next() {
		var item model.UserContextItem
		var source *string
		if err := rows.Scan(&item.UserID, &item.Key, &item.Value, &source, &item.Confidence, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user_context: %w", err)
		}
		if source != nil {
			item.Source = model.ContextSource(*source)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetByKey retrieves a single context item by user and key.
func (s *UserContextStore) GetByKey(ctx context.Context, userID uuid.UUID, key string) (*model.UserContextItem, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("pool not initialized")
	}
	var item model.UserContextItem
	var source *string
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, key, value, source, confidence, updated_at
		 FROM user_context WHERE user_id = $1 AND key = $2`,
		userID, key).Scan(&item.UserID, &item.Key, &item.Value, &source, &item.Confidence, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user_context by key: %w", err)
	}
	if source != nil {
		item.Source = model.ContextSource(*source)
	}
	return &item, nil
}

// Upsert creates or updates a user context item.
func (s *UserContextStore) Upsert(ctx context.Context, item model.UserContextItem) error {
	if s.pool == nil {
		return fmt.Errorf("pool not initialized")
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_context (user_id, key, value, source, confidence, updated_at)
		 VALUES ($1, $2, $3, $4, $5, now())
		 ON CONFLICT (user_id, key) DO UPDATE SET
		   value = EXCLUDED.value,
		   source = EXCLUDED.source,
		   confidence = EXCLUDED.confidence,
		   updated_at = now()`,
		item.UserID, item.Key, item.Value, item.Source, item.Confidence)
	if err != nil {
		return fmt.Errorf("upsert user_context: %w", err)
	}
	return nil
}

// Delete removes a user context item.
func (s *UserContextStore) Delete(ctx context.Context, userID uuid.UUID, key string) error {
	if s.pool == nil {
		return fmt.Errorf("pool not initialized")
	}
	_, err := s.pool.Exec(ctx,
		`DELETE FROM user_context WHERE user_id = $1 AND key = $2`,
		userID, key)
	if err != nil {
		return fmt.Errorf("delete user_context: %w", err)
	}
	return nil
}
