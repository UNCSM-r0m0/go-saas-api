package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserPreferences represents a user's customization preferences
type UserPreferences struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Profession  string    `json:"profession"`
	Traits      []string  `json:"traits"`
	AboutMe     string    `json:"about_me"`
	Theme       string    `json:"theme"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PreferencesRepository defines the interface for user preferences persistence
type PreferencesRepository interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*UserPreferences, error)
	Upsert(ctx context.Context, prefs *UserPreferences) error
}

// PostgresPreferencesStore implements PreferencesRepository using PostgreSQL
type PostgresPreferencesStore struct {
	pool *pgxpool.Pool
}

// NewPostgresPreferencesStore creates a new Postgres preferences store
func NewPostgresPreferencesStore(pool *pgxpool.Pool) *PostgresPreferencesStore {
	return &PostgresPreferencesStore{pool: pool}
}

// GetByUserID retrieves preferences by user ID
func (s *PostgresPreferencesStore) GetByUserID(ctx context.Context, userID uuid.UUID) (*UserPreferences, error) {
	query := `
		SELECT id, user_id, display_name, profession, traits, about_me, theme, created_at, updated_at
		FROM user_preferences WHERE user_id = $1
	`
	row := s.pool.QueryRow(ctx, query, userID)
	return scanPreferences(row)
}

// Upsert creates or updates user preferences
func (s *PostgresPreferencesStore) Upsert(ctx context.Context, prefs *UserPreferences) error {
	query := `
		INSERT INTO user_preferences (id, user_id, display_name, profession, traits, about_me, theme, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			profession = EXCLUDED.profession,
			traits = EXCLUDED.traits,
			about_me = EXCLUDED.about_me,
			theme = EXCLUDED.theme,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.pool.Exec(ctx, query,
		prefs.ID, prefs.UserID, prefs.DisplayName, prefs.Profession,
		prefs.Traits, prefs.AboutMe, prefs.Theme, prefs.CreatedAt, prefs.UpdatedAt,
	)
	return err
}

func scanPreferences(row pgx.Row) (*UserPreferences, error) {
	var p UserPreferences
	var traits []string
	err := row.Scan(
		&p.ID, &p.UserID, &p.DisplayName, &p.Profession,
		&traits, &p.AboutMe, &p.Theme, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.Traits = traits
	return &p, nil
}
