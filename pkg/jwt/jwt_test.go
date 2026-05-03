package jwt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndValidateToken(t *testing.T) {
	mgr := NewManager("test-secret-key-for-unit-tests-only")
	userID := uuid.NewString()

	token, err := mgr.GenerateToken(userID, "member", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := mgr.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "member", claims.Role)
	assert.NotEmpty(t, claims.ID)
}

func TestValidateToken_Invalid(t *testing.T) {
	mgr := NewManager("test-secret-key")

	_, err := mgr.ValidateToken("not-a-valid-token")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_WrongSecret(t *testing.T) {
	mgr1 := NewManager("secret-one")
	mgr2 := NewManager("secret-two")

	token, err := mgr1.GenerateToken("user-1", "member", time.Hour)
	require.NoError(t, err)

	_, err = mgr2.ValidateToken(token)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_Expired(t *testing.T) {
	mgr := NewManager("test-secret-key")

	token, err := mgr.GenerateToken("user-1", "member", -time.Hour)
	require.NoError(t, err)

	_, err = mgr.ValidateToken(token)
	assert.ErrorIs(t, err, ErrExpiredToken)
}

func TestParseUserID(t *testing.T) {
	mgr := NewManager("test-secret-key")
	userID := uuid.NewString()

	token, err := mgr.GenerateToken(userID, "admin", time.Hour)
	require.NoError(t, err)

	parsed, err := mgr.ParseUserID(token)
	require.NoError(t, err)
	assert.Equal(t, userID, parsed)
}
