package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordResetService(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	require.NotNil(t, service, "NewPasswordResetService() should not return nil")
}

func TestPasswordResetService_RequestPasswordReset_Success(t *testing.T) {
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{
			"user@example.com": {
				ID:     456,
				Email:  "user@example.com",
				Name:   "TestUser",
				Status: "verified",
			},
		},
	}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	result, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.NoError(t, err, "RequestPasswordReset() should succeed")
	require.NotNil(t, result, "RequestPasswordReset() should return result")
	assert.NotEmpty(t, result.Token, "Token should not be empty")
	assert.Equal(t, 456, result.UserID, "Expected UserID 456")
	assert.Equal(t, "user@example.com", result.Email, "Expected email to match")
	assert.Equal(t, "TestUser", result.Name, "Expected name to match")
}

func TestPasswordResetService_RequestPasswordReset_UserNotFound(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	result, err := service.RequestPasswordReset(context.Background(), "nonexistent@example.com")

	require.NoError(t, err, "RequestPasswordReset() should not error for non-existent user")
	assert.Nil(t, result, "RequestPasswordReset() should return nil for non-existent user")
}

func TestPasswordResetService_RequestPasswordReset_GetUserByEmailError(t *testing.T) {
	store := &stubUserStore{getErr: errors.New("database error")}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	_, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.Error(t, err, "Expected error when GetUserByEmail fails")
	assert.ErrorIs(t, err, authdomain.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_RequestPasswordReset_DeleteTokensFailure(t *testing.T) {
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{
			"user@example.com": {
				ID:     456,
				Email:  "user@example.com",
				Status: "verified",
			},
		},
	}
	tokens := &stubResetStore{deleteErr: errors.New("database error")}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	_, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.Error(t, err, "Expected error when DeleteUserTokens fails")
	assert.ErrorIs(t, err, authdomain.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_RequestPasswordReset_CreateTokenFailure(t *testing.T) {
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{
			"user@example.com": {
				ID:     456,
				Email:  "user@example.com",
				Status: "verified",
			},
		},
	}
	tokens := &stubResetStore{createErr: errors.New("database error")}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	_, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.Error(t, err, "Expected error when CreateToken fails")
	assert.ErrorIs(t, err, authdomain.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_ResetPassword_Success(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{
		token: &authdomain.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	result, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.NoError(t, err, "ResetPassword() should succeed")
	require.NotNil(t, result, "ResetPassword() should return result")
	assert.True(t, result.Success, "Expected Success to be true")
	assert.NotEmpty(t, result.Message, "Message should not be empty")
	assert.Equal(t, 456, result.UserID, "Expected UserID 456")
}

func TestPasswordResetService_ResetPassword_InvalidPassword(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{passwordErr: errors.New("weak password")}, 1)

	_, err := service.ResetPassword(context.Background(), "test-token", "weak")

	require.Error(t, err, "Expected error for invalid password")
}

func TestPasswordResetService_ResetPassword_InvalidToken(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	_, err := service.ResetPassword(context.Background(), "invalid-token", "NewP@ssw0rd!12345")

	assert.ErrorIs(t, err, authdomain.ErrResetTokenInvalid, "Expected ErrResetTokenInvalid")
}

func TestPasswordResetService_ResetPassword_ExpiredToken(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{
		token: &authdomain.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	_, err := service.ResetPassword(context.Background(), "expired-token", "NewP@ssw0rd!12345")

	assert.ErrorIs(t, err, authdomain.ErrResetTokenExpired, "Expected ErrResetTokenExpired")
}

func TestPasswordResetService_ResetPassword_FindTokenError(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{findErr: errors.New("database error")}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	_, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.Error(t, err, "Expected error when FindValidToken fails")
	assert.ErrorIs(t, err, authdomain.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_ResetPassword_UpdatePasswordError(t *testing.T) {
	store := &stubUserStore{updateErr: errors.New("database error")}
	tokens := &stubResetStore{
		token: &authdomain.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	_, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.Error(t, err, "Expected error when UpdatePasswordByUserID fails")
	assert.ErrorIs(t, err, authdomain.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_RequestPasswordReset_UserNotVerified(t *testing.T) {
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{
			"user@example.com": {
				ID:     456,
				Email:  "user@example.com",
				Status: "pending",
			},
		},
	}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	result, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.NoError(t, err, "RequestPasswordReset() should not error for non-verified user")
	assert.Nil(t, result, "RequestPasswordReset() should return nil for non-verified user")
}

func TestPasswordResetService_RequestPasswordReset_SuspendedUser(t *testing.T) {
	store := &stubUserStore{
		byEmail: map[string]*userdomain.User{
			"user@example.com": {
				ID:     456,
				Email:  "user@example.com",
				Status: "suspended",
			},
		},
	}
	tokens := &stubResetStore{}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	result, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.NoError(t, err, "RequestPasswordReset() should not error for suspended user")
	assert.Nil(t, result, "RequestPasswordReset() should return nil for suspended user")
}

func TestPasswordResetService_ResetPassword_DeleteTokenAfterReset(t *testing.T) {
	store := &stubUserStore{}
	tokens := &stubResetStore{
		token: &authdomain.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
		deleteErr: errors.New("ignored error"),
	}

	service := authdomain.NewPasswordResetService(store, tokens, stubPolicy{}, 1)

	result, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.NoError(t, err, "ResetPassword() should succeed even if DeleteUserTokens fails")
	assert.True(t, result.Success, "Expected Success to be true")
}
