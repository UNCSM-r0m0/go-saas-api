package auth

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated user in the system
type User struct {
	ID                    uuid.UUID `json:"id"`
	Email                 string    `json:"email"`
	PasswordHash          string    `json:"-"` // never serialized
	Name                  string    `json:"name"`
	Role                  string    `json:"role"`
	IsAdmin               bool      `json:"is_admin"`
	MessagesUsedThisMonth int       `json:"messages_used_this_month"`
	OAuthProvider         string    `json:"oauth_provider,omitempty"`
	OAuthSubject          string    `json:"oauth_subject,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// RegisterRequest represents a user registration payload
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
}

// LoginRequest represents a user login payload
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

// RefreshRequest represents a token refresh payload
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ForgotPasswordRequest represents a password reset request payload
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents a password reset confirmation payload
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// MeResponse represents the current user response
type MeResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token,omitempty"`
}
