package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/r0lm0/go-saas-api/pkg/jwt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("invalid refresh token")
	ErrInvalidResetToken  = errors.New("invalid or expired reset token")
)

// RefreshTokenStore defines the interface for refresh token persistence
type RefreshTokenStore interface {
	Save(ctx context.Context, userID, token string, expiration time.Duration) error
	Get(ctx context.Context, token string) (string, error) // returns userID
	Delete(ctx context.Context, token string) error
}

// Service handles authentication business logic
type Service struct {
	users         UserRepository
	refreshTokens RefreshTokenStore
	resetTokens   PasswordResetTokenStore
	emailSender   EmailSender
	frontendURL   string
	jwtManager    *jwt.Manager
	accessTTL     time.Duration
	refreshTTL    time.Duration
	resetTokenTTL time.Duration
}

// NewService creates a new auth service
func NewService(users UserRepository, refresh RefreshTokenStore, reset PasswordResetTokenStore, email EmailSender, frontendURL string, jwtMgr *jwt.Manager, accessTTL, refreshTTL, resetTokenTTL time.Duration) *Service {
	return &Service{
		users:         users,
		refreshTokens: refresh,
		resetTokens:   reset,
		emailSender:   email,
		frontendURL:   frontendURL,
		jwtManager:    jwtMgr,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		resetTokenTTL: resetTokenTTL,
	}
}

// Register creates a new user with a hashed password
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
	// Check if user already exists
	existing, err := s.users.GetByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	user := &User{
		ID:                    uuid.New(),
		Email:                 req.Email,
		PasswordHash:          string(hash),
		Name:                  req.Name,
		Role:                  "member",
		IsAdmin:               false,
		MessagesUsedThisMonth: 0,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// Login validates credentials and returns a token pair
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*TokenPair, *User, error) {
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return pair, user, nil
}

// Refresh generates a new token pair from a valid refresh token
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, *User, error) {
	userIDStr, err := s.refreshTokens.Get(ctx, refreshToken)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	// Rotate: delete old refresh token
	_ = s.refreshTokens.Delete(ctx, refreshToken)

	pair, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return pair, user, nil
}

// Logout invalidates a refresh token
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.refreshTokens.Delete(ctx, refreshToken)
}

// GetUserByID retrieves a user by their ID
func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.users.GetByID(ctx, id)
}

// RequestPasswordReset generates a reset token and sends an email to the user
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Do not reveal whether the email exists for security
		return nil
	}

	token, err := GeneratePasswordResetToken()
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	if err := s.resetTokens.Save(ctx, token, user.ID.String(), s.resetTokenTTL); err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.frontendURL, token)
	if err := s.emailSender.SendPasswordReset(user.Email, resetURL); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	return nil
}

// ResetPassword validates a reset token and updates the user's password
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	userIDStr, err := s.resetTokens.Get(ctx, token)
	if err != nil {
		return ErrInvalidResetToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return ErrInvalidResetToken
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return ErrInvalidResetToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user.PasswordHash = string(hash)
	user.UpdatedAt = time.Now().UTC()

	if err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("update user password: %w", err)
	}

	_ = s.resetTokens.Delete(ctx, token)
	return nil
}

func (s *Service) generateTokenPair(ctx context.Context, user *User) (*TokenPair, error) {
	accessToken, err := s.jwtManager.GenerateToken(user.ID.String(), user.Role, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken := uuid.NewString()
	if err := s.refreshTokens.Save(ctx, user.ID.String(), refreshToken, s.refreshTTL); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}
