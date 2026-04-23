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
	jwtManager    *jwt.Manager
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// NewService creates a new auth service
func NewService(users UserRepository, refresh RefreshTokenStore, jwtMgr *jwt.Manager, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		users:         users,
		refreshTokens: refresh,
		jwtManager:    jwtMgr,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

// Register creates a new user with a hashed password
func (s *Service) Register(ctx context.Context, tenantID uuid.UUID, req *RegisterRequest) (*User, error) {
	// Check if user already exists
	existing, err := s.users.GetByEmail(ctx, tenantID, req.Email)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	user := &User{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Email:        req.Email,
		PasswordHash: string(hash),
		Name:         req.Name,
		Role:         "member",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// Login validates credentials and returns a token pair
func (s *Service) Login(ctx context.Context, tenantID uuid.UUID, req *LoginRequest) (*TokenPair, *User, error) {
	user, err := s.users.GetByEmail(ctx, tenantID, req.Email)
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

func (s *Service) generateTokenPair(ctx context.Context, user *User) (*TokenPair, error) {
	accessToken, err := s.jwtManager.GenerateToken(user.ID.String(), user.TenantID.String(), user.Role, s.accessTTL)
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
