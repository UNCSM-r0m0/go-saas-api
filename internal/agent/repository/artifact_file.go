package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
)

// ArtifactFileRepo defines persistence for artifact files.
type ArtifactFileRepo interface {
	Create(ctx context.Context, file *model.ArtifactFile) error
	CreateMany(ctx context.Context, files []model.ArtifactFile) error
	GetByArtifactID(ctx context.Context, artifactID uuid.UUID) ([]model.ArtifactFile, error)
	GetByPath(ctx context.Context, artifactID uuid.UUID, path string) (*model.ArtifactFile, error)
	DeleteByArtifactID(ctx context.Context, artifactID uuid.UUID) error
}

// PostgresArtifactFileRepo implements ArtifactFileRepo with PostgreSQL.
type PostgresArtifactFileRepo struct {
	pool *pgxpool.Pool
}

// NewArtifactFileRepo creates a new PostgresArtifactFileRepo.
func NewArtifactFileRepo(pool *pgxpool.Pool) *PostgresArtifactFileRepo {
	return &PostgresArtifactFileRepo{pool: pool}
}

// Create inserts a single artifact file.
func (r *PostgresArtifactFileRepo) Create(ctx context.Context, file *model.ArtifactFile) error {
	query := `
		INSERT INTO artifact_files (id, artifact_id, path, language, content, file_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		file.ID, file.ArtifactID, file.Path, file.Language, file.Content,
		file.FileOrder, file.CreatedAt, file.UpdatedAt,
	)
	return err
}

// CreateMany inserts multiple artifact files in a single transaction.
func (r *PostgresArtifactFileRepo) CreateMany(ctx context.Context, files []model.ArtifactFile) error {
	if len(files) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, f := range files {
		batch.Queue(`
			INSERT INTO artifact_files (id, artifact_id, path, language, content, file_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, f.ID, f.ArtifactID, f.Path, f.Language, f.Content, f.FileOrder, f.CreatedAt, f.UpdatedAt)
	}

	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("execute batch: %w", err)
	}

	return tx.Commit(ctx)
}

// GetByArtifactID retrieves all files for an artifact.
func (r *PostgresArtifactFileRepo) GetByArtifactID(ctx context.Context, artifactID uuid.UUID) ([]model.ArtifactFile, error) {
	query := `
		SELECT id, artifact_id, path, language, content, file_order, created_at, updated_at
		FROM artifact_files
		WHERE artifact_id = $1
		ORDER BY file_order ASC, path ASC
	`
	rows, err := r.pool.Query(ctx, query, artifactID)
	if err != nil {
		return nil, fmt.Errorf("query artifact files: %w", err)
	}
	defer rows.Close()

	var files []model.ArtifactFile
	for rows.Next() {
		var f model.ArtifactFile
		if err := rows.Scan(
			&f.ID, &f.ArtifactID, &f.Path, &f.Language, &f.Content,
			&f.FileOrder, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan artifact file: %w", err)
		}
		files = append(files, f)
	}

	return files, rows.Err()
}

// GetByPath retrieves a single file by artifact ID and path.
func (r *PostgresArtifactFileRepo) GetByPath(ctx context.Context, artifactID uuid.UUID, path string) (*model.ArtifactFile, error) {
	query := `
		SELECT id, artifact_id, path, language, content, file_order, created_at, updated_at
		FROM artifact_files
		WHERE artifact_id = $1 AND path = $2
	`
	var f model.ArtifactFile
	err := r.pool.QueryRow(ctx, query, artifactID, path).Scan(
		&f.ID, &f.ArtifactID, &f.Path, &f.Language, &f.Content,
		&f.FileOrder, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query artifact file: %w", err)
	}
	return &f, nil
}

// DeleteByArtifactID deletes all files for an artifact.
func (r *PostgresArtifactFileRepo) DeleteByArtifactID(ctx context.Context, artifactID uuid.UUID) error {
	query := `DELETE FROM artifact_files WHERE artifact_id = $1`
	_, err := r.pool.Exec(ctx, query, artifactID)
	return err
}
