package auth

import (
	"context"

	"github.com/google/uuid"
)

// UserRepository defines the interface for user persistence
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByOAuth(ctx context.Context, provider, subject string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListUsers(ctx context.Context, limit, offset int) ([]*User, error)
	UpdateUserRole(ctx context.Context, id uuid.UUID, role string) error
}
