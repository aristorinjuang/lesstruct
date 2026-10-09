package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVerificationService(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{}

	service := authdomain.NewVerificationService(store, tokens, 24)

	require.NotNil(t, service, "NewVerificationService() should not return nil")
}

func TestVerificationService_CreateVerificationToken_Success(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{}

	service := authdomain.NewVerificationService(store, tokens, 24)

	result, err := service.CreateVerificationToken(context.Background(), 456)

	require.NoError(t, err, "CreateVerificationToken() should succeed")
	require.NotNil(t, result, "CreateVerificationToken() should return result")

	assert.NotEmpty(t, result.Token, "Token should not be empty")
	assert.Equal(t, 456, result.UserID, "Expected UserID 456")
	assert.Len(t, result.Token, 64, "Token length should be 64 (32 bytes * 2 for hex encoding)")
}

func TestVerificationService_CreateVerificationToken_DeleteOldTokensFailure(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{deleteErr: errors.New("database error")}

	service := authdomain.NewVerificationService(store, tokens, 24)

	_, err := service.CreateVerificationToken(context.Background(), 456)

	require.Error(t, err, "Expected error when DeleteUserTokens fails")
}

func TestVerificationService_CreateVerificationToken_CreateTokenFailure(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{createErr: errors.New("database error")}

	service := authdomain.NewVerificationService(store, tokens, 24)

	_, err := service.CreateVerificationToken(context.Background(), 456)

	require.Error(t, err, "Expected error when CreateToken fails")
}

func TestVerificationService_VerifyEmail_Success(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{
		token: &authdomain.VerificationToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	service := authdomain.NewVerificationService(store, tokens, 24)

	result, err := service.VerifyEmail(context.Background(), "test-token")

	require.NoError(t, err, "VerifyEmail() should succeed")
	require.NotNil(t, result, "VerifyEmail() should return result")

	assert.True(t, result.Success, "Expected Success to be true")
	assert.Equal(t, "Email verified successfully", result.Message)
	assert.True(t, store.statusUpdated, "UpdateUserStatus must run on success")
}

func TestVerificationService_VerifyEmail_AdminApprovalRequired(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{
		token: &authdomain.VerificationToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	service := authdomain.NewVerificationService(store, tokens, 24, authdomain.WithAdminApprovalRequired())

	result, err := service.VerifyEmail(context.Background(), "test-token")

	require.NoError(t, err, "VerifyEmail() should succeed")
	require.NotNil(t, result, "VerifyEmail() should return result")

	assert.True(t, result.Success, "Expected Success to be true")
	assert.Equal(t, "Email verified. An administrator will activate your account.", result.Message)
	// The account must stay pending: only admin approval may activate it.
	assert.False(t, store.statusUpdated, "UpdateUserStatus must not run in admin-approval mode")
}

func TestVerificationService_VerifyEmail_AdminApprovalRequired_SetEmailVerifiedError(t *testing.T) {
	store := &stubUserStore{updateErr: errors.New("database connection lost")}
	tokens := &stubTokenStore{
		token: &authdomain.VerificationToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	service := authdomain.NewVerificationService(store, tokens, 24, authdomain.WithAdminApprovalRequired())

	_, err := service.VerifyEmail(context.Background(), "test-token")

	require.Error(t, err, "Expected error when SetEmailVerified fails")
	assert.ErrorIs(t, err, authdomain.ErrVerificationFailed, "Expected ErrVerificationFailed")
}

func TestVerificationService_VerifyEmail_InvalidToken(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{}

	service := authdomain.NewVerificationService(store, tokens, 24)

	_, err := service.VerifyEmail(context.Background(), "invalid-token")

	assert.ErrorIs(t, err, authdomain.ErrTokenInvalid, "Expected ErrTokenInvalid")
}

func TestVerificationService_VerifyEmail_ExpiredToken(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{
		token: &authdomain.VerificationToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}

	service := authdomain.NewVerificationService(store, tokens, 24)

	_, err := service.VerifyEmail(context.Background(), "expired-token")

	assert.ErrorIs(t, err, authdomain.ErrTokenExpired, "Expected ErrTokenExpired")
}

func TestVerificationService_CleanupExpiredTokens(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubTokenStore{}

	service := authdomain.NewVerificationService(store, tokens, 24)

	err := service.CleanupExpiredTokens(context.Background())

	assert.NoError(t, err, "CleanupExpiredTokens() should succeed")
}
