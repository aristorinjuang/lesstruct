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

func TestNewVerificationService(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	require.NotNil(t, service, "NewVerificationService() should not return nil")
}

func TestVerificationService_CreateVerificationToken_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(nil)

	tokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Run(func(_ context.Context, token *repository.VerificationToken) {
			token.ID = 123
		}).
		Return(nil)

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	result, err := service.CreateVerificationToken(context.Background(), 456)

	require.NoError(t, err, "CreateVerificationToken() should succeed")
	require.NotNil(t, result, "CreateVerificationToken() should return result")

	assert.NotEmpty(t, result.Token, "Token should not be empty")
	assert.Equal(t, 456, result.UserID, "Expected UserID 456")
	assert.Len(t, result.Token, 64, "Token length should be 64 (32 bytes * 2 for hex encoding)")
}

func TestVerificationService_CreateVerificationToken_DeleteOldTokensFailure(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(errors.New("database error"))

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	_, err := service.CreateVerificationToken(context.Background(), 456)

	require.Error(t, err, "Expected error when DeleteUserTokens fails")
}

func TestVerificationService_CreateVerificationToken_CreateTokenFailure(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(nil)

	tokenRepo.EXPECT().
		CreateToken(mock.Anything, mock.Anything).
		Return(errors.New("database error"))

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	_, err := service.CreateVerificationToken(context.Background(), 456)

	require.Error(t, err, "Expected error when CreateToken fails")
}

func TestVerificationService_VerifyEmail_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.VerificationToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}, nil)

	userRepo.EXPECT().
		UpdateUserStatus(
			mock.Anything,
			456,
			"verified",
		).
		Return(nil)

	tokenRepo.EXPECT().
		DeleteUserTokens(mock.Anything, 456).
		Return(nil)

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	result, err := service.VerifyEmail(context.Background(), "test-token")

	require.NoError(t, err, "VerifyEmail() should succeed")
	require.NotNil(t, result, "VerifyEmail() should return result")

	assert.True(t, result.Success, "Expected Success to be true")
	assert.NotEmpty(t, result.Message, "Message should not be empty")
}

func TestVerificationService_VerifyEmail_InvalidToken(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(nil, nil) // Token not found

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	_, err := service.VerifyEmail(context.Background(), "invalid-token")

	assert.ErrorIs(t, err, auth.ErrTokenInvalid, "Expected ErrTokenInvalid")
}

func TestVerificationService_VerifyEmail_ExpiredToken(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.VerificationToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		}, nil)

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	_, err := service.VerifyEmail(context.Background(), "expired-token")

	assert.ErrorIs(t, err, auth.ErrTokenExpired, "Expected ErrTokenExpired")
}

func TestVerificationService_CleanupExpiredTokens(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		DeleteExpiredTokens(mock.Anything).
		Return(nil)

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	err := service.CleanupExpiredTokens(context.Background())

	assert.NoError(t, err, "CleanupExpiredTokens() should succeed")
}

func TestVerificationService_VerifyEmail_FindValidTokenError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(nil, errors.New("database connection lost"))

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	_, err := service.VerifyEmail(context.Background(), "test-token")

	require.Error(t, err, "Expected error when FindValidToken fails")
	assert.ErrorIs(t, err, auth.ErrVerificationFailed, "Expected ErrVerificationFailed")
}

func TestVerificationService_VerifyEmail_UpdateUserStatusError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	tokenRepo := repomocks.NewMockVerificationTokenRepo(t)

	tokenRepo.EXPECT().
		FindValidToken(mock.Anything, mock.Anything).
		Return(&repository.VerificationToken{
			ID:        123,
			UserID:    456,
			TokenHash: "test-hash",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}, nil)

	userRepo.EXPECT().
		UpdateUserStatus(
			mock.Anything,
			456,
			"verified",
		).
		Return(errors.New("database connection lost"))

	service := auth.NewVerificationService(userRepo, tokenRepo, 24)

	_, err := service.VerifyEmail(context.Background(), "test-token")

	require.Error(t, err, "Expected error when UpdateUserStatus fails")
	assert.ErrorIs(t, err, auth.ErrVerificationFailed, "Expected ErrVerificationFailed")
}
