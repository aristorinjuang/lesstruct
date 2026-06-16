package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordResetService(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	require.NotNil(t, service, "NewPasswordResetService() should not return nil")
}

func TestPasswordResetService_RequestPasswordReset_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "user@example.com").
		Return(&repository.User{
			ID:     456,
			Email:  "user@example.com",
			Name:   "TestUser",
			Status: "verified",
		}, nil)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(nil)

	tokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Run(func(_ context.Context, token *repository.PasswordResetToken) {
			token.ID = 123
		}).
		Return(nil)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	result, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.NoError(t, err, "RequestPasswordReset() should succeed")
	require.NotNil(t, result, "RequestPasswordReset() should return result")
	assert.NotEmpty(t, result.Token, "Token should not be empty")
	assert.Equal(t, 456, result.UserID, "Expected UserID 456")
	assert.Equal(t, "user@example.com", result.Email, "Expected email to match")
	assert.Equal(t, "TestUser", result.Name, "Expected name to match")
}

func TestPasswordResetService_RequestPasswordReset_UserNotFound(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "nonexistent@example.com").
		Return(nil, nil)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	result, err := service.RequestPasswordReset(context.Background(), "nonexistent@example.com")

	require.NoError(t, err, "RequestPasswordReset() should not error for non-existent user")
	assert.Nil(t, result, "RequestPasswordReset() should return nil for non-existent user")
}

func TestPasswordResetService_RequestPasswordReset_GetUserByEmailError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "user@example.com").
		Return(nil, errors.New("database error"))

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.Error(t, err, "Expected error when GetUserByEmail fails")
	assert.ErrorIs(t, err, auth.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_RequestPasswordReset_DeleteTokensFailure(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "user@example.com").
		Return(&repository.User{
			ID:     456,
			Email:  "user@example.com",
			Status: "verified",
		}, nil)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(errors.New("database error"))

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.Error(t, err, "Expected error when DeleteUserTokens fails")
	assert.ErrorIs(t, err, auth.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_RequestPasswordReset_CreateTokenFailure(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "user@example.com").
		Return(&repository.User{
			ID:     456,
			Email:  "user@example.com",
			Status: "verified",
		}, nil)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(nil)

	tokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Return(errors.New("database error"))

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.Error(t, err, "Expected error when CreateToken fails")
	assert.ErrorIs(t, err, auth.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_ResetPassword_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}, nil)

	userRepo.EXPECT().
		UpdatePasswordByUserID(mock.Anything, 456, mock.Anything).
		Return(nil)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(nil)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	result, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.NoError(t, err, "ResetPassword() should succeed")
	require.NotNil(t, result, "ResetPassword() should return result")
	assert.True(t, result.Success, "Expected Success to be true")
	assert.NotEmpty(t, result.Message, "Message should not be empty")
	assert.Equal(t, 456, result.UserID, "Expected UserID 456")
}

func TestPasswordResetService_ResetPassword_InvalidPassword(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.ResetPassword(context.Background(), "test-token", "weak")

	require.Error(t, err, "Expected error for invalid password")
}

func TestPasswordResetService_ResetPassword_InvalidToken(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(nil, nil)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.ResetPassword(context.Background(), "invalid-token", "NewP@ssw0rd!12345")

	assert.ErrorIs(t, err, auth.ErrResetTokenInvalid, "Expected ErrResetTokenInvalid")
}

func TestPasswordResetService_ResetPassword_ExpiredToken(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}, nil)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.ResetPassword(context.Background(), "expired-token", "NewP@ssw0rd!12345")

	assert.ErrorIs(t, err, auth.ErrResetTokenExpired, "Expected ErrResetTokenExpired")
}

func TestPasswordResetService_ResetPassword_FindTokenError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(nil, errors.New("database error"))

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.Error(t, err, "Expected error when FindValidToken fails")
	assert.ErrorIs(t, err, auth.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_ResetPassword_UpdatePasswordError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}, nil)

	userRepo.EXPECT().
		UpdatePasswordByUserID(mock.Anything, 456, mock.Anything).
		Return(errors.New("database error"))

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	_, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.Error(t, err, "Expected error when UpdatePasswordByUserID fails")
	assert.ErrorIs(t, err, auth.ErrResetFailed, "Expected ErrResetFailed")
}

func TestPasswordResetService_RequestPasswordReset_UserNotVerified(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "user@example.com").
		Return(&repository.User{
			ID:     456,
			Email:  "user@example.com",
			Status: "pending",
		}, nil)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	result, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.NoError(t, err, "RequestPasswordReset() should not error for non-verified user")
	assert.Nil(t, result, "RequestPasswordReset() should return nil for non-verified user")
}

func TestPasswordResetService_RequestPasswordReset_SuspendedUser(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	userRepo.EXPECT().
		GetUserByEmail(mock.Anything, "user@example.com").
		Return(&repository.User{
			ID:     456,
			Email:  "user@example.com",
			Status: "suspended",
		}, nil)

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	result, err := service.RequestPasswordReset(context.Background(), "user@example.com")

	require.NoError(t, err, "RequestPasswordReset() should not error for suspended user")
	assert.Nil(t, result, "RequestPasswordReset() should return nil for suspended user")
}

func TestPasswordResetService_ResetPassword_DeleteTokenAfterReset(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockPasswordResetTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.PasswordResetToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}, nil)

	userRepo.EXPECT().
		UpdatePasswordByUserID(mock.Anything, 456, mock.Anything).
		Return(nil)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(errors.New("ignored error"))

	service := auth.NewPasswordResetService(userRepo, tokenRepo, 1)

	result, err := service.ResetPassword(context.Background(), "test-token", "NewP@ssw0rd!12345")

	require.NoError(t, err, "ResetPassword() should succeed even if DeleteUserTokens fails")
	assert.True(t, result.Success, "Expected Success to be true")
}
