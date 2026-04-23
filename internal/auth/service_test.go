package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUserStore is an in-memory UserRepository for testing
type mockUserStore struct {
	users []User
}

func (m *mockUserStore) Create(ctx context.Context, user *User) error {
	for _, u := range m.users {
		if u.TenantID == user.TenantID && u.Email == user.Email {
			return errors.New("duplicate")
		}
	}
	m.users = append(m.users, *user)
	return nil
}

func (m *mockUserStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockUserStore) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	for _, u := range m.users {
		if u.TenantID == tenantID && u.Email == email {
			return &u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockUserStore) GetByOAuth(ctx context.Context, provider, subject string) (*User, error) {
	for _, u := range m.users {
		if u.OAuthProvider == provider && u.OAuthSubject == subject {
			return &u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockUserStore) Update(ctx context.Context, user *User) error {
	for i, u := range m.users {
		if u.ID == user.ID {
			m.users[i] = *user
			return nil
		}
	}
	return errors.New("not found")
}

func (m *mockUserStore) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

// mockRefreshStore is an in-memory RefreshTokenStore for testing
type mockRefreshStore struct {
	tokens map[string]string // token -> userID
}

func (m *mockRefreshStore) Save(ctx context.Context, userID, token string, expiration time.Duration) error {
	if m.tokens == nil {
		m.tokens = make(map[string]string)
	}
	m.tokens[token] = userID
	return nil
}

func (m *mockRefreshStore) Get(ctx context.Context, token string) (string, error) {
	if m.tokens == nil {
		return "", errors.New("not found")
	}
	uid, ok := m.tokens[token]
	if !ok {
		return "", errors.New("not found")
	}
	return uid, nil
}

func (m *mockRefreshStore) Delete(ctx context.Context, token string) error {
	if m.tokens != nil {
		delete(m.tokens, token)
	}
	return nil
}

func newTestService() (*Service, *mockUserStore, *mockRefreshStore) {
	users := &mockUserStore{}
	refresh := &mockRefreshStore{}
	jwtMgr := jwt.NewManager("test-secret")
	svc := NewService(users, refresh, jwtMgr, time.Hour, 24*time.Hour)
	return svc, users, refresh
}

func TestService_Register(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	user, err := svc.Register(ctx, tenantID, &RegisterRequest{
		Email:    "test@example.com",
		Password: "securepassword123",
		Name:     "Test User",
	})
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "member", user.Role)
	assert.NotEqual(t, uuid.Nil, user.ID)
}

func TestService_Register_Duplicate(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	_, err := svc.Register(ctx, tenantID, &RegisterRequest{
		Email:    "dup@example.com",
		Password: "password123",
		Name:     "First",
	})
	require.NoError(t, err)

	_, err = svc.Register(ctx, tenantID, &RegisterRequest{
		Email:    "dup@example.com",
		Password: "password456",
		Name:     "Second",
	})
	assert.ErrorIs(t, err, ErrUserExists)
}

func TestService_Login(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	_, err := svc.Register(ctx, tenantID, &RegisterRequest{
		Email:    "login@example.com",
		Password: "mypassword",
		Name:     "Login User",
	})
	require.NoError(t, err)

	pair, user, err := svc.Login(ctx, tenantID, &LoginRequest{
		Email:    "login@example.com",
		Password: "mypassword",
	})
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Greater(t, pair.ExpiresIn, int64(0))
}

func TestService_Login_InvalidCredentials(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	_, err := svc.Register(ctx, tenantID, &RegisterRequest{
		Email:    "bad@example.com",
		Password: "correct",
		Name:     "Bad User",
	})
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, tenantID, &LoginRequest{
		Email:    "bad@example.com",
		Password: "wrong",
	})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestService_Refresh(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	_, err := svc.Register(ctx, tenantID, &RegisterRequest{
		Email:    "refresh@example.com",
		Password: "password",
		Name:     "Refresh User",
	})
	require.NoError(t, err)

	pair, _, err := svc.Login(ctx, tenantID, &LoginRequest{
		Email:    "refresh@example.com",
		Password: "password",
	})
	require.NoError(t, err)

	newPair, user, err := svc.Refresh(ctx, pair.RefreshToken)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, newPair.AccessToken)
	assert.NotEqual(t, pair.AccessToken, newPair.AccessToken)
}

func TestService_Refresh_InvalidToken(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	_, _, err := svc.Refresh(ctx, "invalid-token")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestService_Logout(t *testing.T) {
	svc, _, refresh := newTestService()
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	_, err := svc.Register(ctx, tenantID, &RegisterRequest{
		Email:    "logout@example.com",
		Password: "password",
		Name:     "Logout User",
	})
	require.NoError(t, err)

	pair, _, err := svc.Login(ctx, tenantID, &LoginRequest{
		Email:    "logout@example.com",
		Password: "password",
	})
	require.NoError(t, err)

	err = svc.Logout(ctx, pair.RefreshToken)
	require.NoError(t, err)

	_, ok := refresh.tokens[pair.RefreshToken]
	assert.False(t, ok)
}
