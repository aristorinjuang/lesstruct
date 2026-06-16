package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		wantError error
	}{
		{
			name:      "Valid username with letters",
			username:  "testuser",
			wantError: nil,
		},
		{
			name:      "Valid username with numbers",
			username:  "user123",
			wantError: nil,
		},
		{
			name:      "Valid username with underscore",
			username:  "test_user",
			wantError: nil,
		},
		{
			name:      "Valid username with hyphen",
			username:  "test-user",
			wantError: nil,
		},
		{
			name:      "Valid username with mixed characters",
			username:  "Test_User-123",
			wantError: nil,
		},
		{
			name:      "Valid single character username",
			username:  "a",
			wantError: nil,
		},
		{
			name:      "Valid 50 character username",
			username:  strings.Repeat("a", 50),
			wantError: nil,
		},
		{
			name:      "Empty username",
			username:  "",
			wantError: auth.ErrUsernameInvalid,
		},
		{
			name:      "Username with spaces",
			username:  "test user",
			wantError: auth.ErrUsernameInvalid,
		},
		{
			name:      "Username with special characters",
			username:  "test@user",
			wantError: auth.ErrUsernameInvalid,
		},
		{
			name:      "Username with dots",
			username:  "test.user",
			wantError: auth.ErrUsernameInvalid,
		},
		{
			name:      "Username with 51 characters (too long)",
			username:  strings.Repeat("a", 51),
			wantError: auth.ErrUsernameInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidateUsername(tt.username)
			if tt.wantError != nil {
				assert.ErrorIs(t, err, tt.wantError, "ValidateUsername() should return expected error")
			} else {
				assert.NoError(t, err, "ValidateUsername() should not return error")
			}
		})
	}
}

func TestRegistrationService_RegisterUser_Success(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	mockRepo.EXPECT().
		CheckUsernameExists(mock.Anything, mock.Anything).
		Return(false, nil)

	mockRepo.EXPECT().
		CheckEmailExists(mock.Anything, mock.Anything).
		Return(false, nil)

	mockRepo.EXPECT().
		CreateUser(mock.Anything, mock.Anything).
		Run(func(_ context.Context, user *repository.User) {
			user.ID = 123
		}).
		Return(nil)

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	result, err := service.RegisterUser(context.Background(), req)
	require.NoError(t, err, "RegisterUser() should succeed")
	require.NotNil(t, result, "RegisterUser() should return result")

	assert.Equal(t, 123, result.UserID, "UserID should match")
	assert.NotEmpty(t, result.Message, "Message should not be empty")
}

func TestRegistrationService_RegisterUser_InvalidUsername(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "test user", // Invalid: contains space
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.ErrorIs(t, err, auth.ErrUsernameInvalid, "Should return ErrUsernameInvalid")
}

func TestRegistrationService_RegisterUser_InvalidEmail(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "testuser",
		Email:    "invalidemail", // Invalid email format
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error for invalid email")
}

func TestRegistrationService_RegisterUser_InvalidPassword(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "weak", // Invalid password
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error for invalid password")
}

func TestRegistrationService_RegisterUser_UsernameExists(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	mockRepo.EXPECT().
		CheckUsernameExists(mock.Anything, mock.Anything).
		Return(true, nil)

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "existinguser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.ErrorIs(t, err, auth.ErrUsernameExists, "Should return ErrUsernameExists")
}

func TestRegistrationService_RegisterUser_EmailExists(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	mockRepo.EXPECT().
		CheckUsernameExists(mock.Anything, mock.Anything).
		Return(false, nil)

	mockRepo.EXPECT().
		CheckEmailExists(mock.Anything, mock.Anything).
		Return(true, nil)

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "testuser",
		Email:    "existing@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.ErrorIs(t, err, auth.ErrEmailExists, "Should return ErrEmailExists")
}

func TestRegistrationService_RegisterUser_CreateUserFailure(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	mockRepo.EXPECT().
		CheckUsernameExists(mock.Anything, mock.Anything).
		Return(false, nil)

	mockRepo.EXPECT().
		CheckEmailExists(mock.Anything, mock.Anything).
		Return(false, nil)

	mockRepo.EXPECT().
		CreateUser(mock.Anything, mock.Anything).
		Return(errors.New("database error"))

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error when CreateUser fails")
	assert.ErrorIs(t, err, auth.ErrRegistrationFailed, "Should wrap error in ErrRegistrationFailed")
}

func TestRegistrationService_RegisterUser_CheckUsernameExistsError(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	mockRepo.EXPECT().
		CheckUsernameExists(mock.Anything, mock.Anything).
		Return(false, errors.New("database connection lost"))

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error when CheckUsernameExists fails")
}

func TestRegistrationService_RegisterUser_CheckEmailExistsError(t *testing.T) {
	mockRepo := repomocks.NewMockUserRepo(t)

	mockRepo.EXPECT().
		CheckUsernameExists(mock.Anything, mock.Anything).
		Return(false, nil)

	mockRepo.EXPECT().
		CheckEmailExists(mock.Anything, mock.Anything).
		Return(false, errors.New("database connection lost"))

	service := auth.NewRegistrationService(mockRepo)

	req := auth.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecurePassword123!",
	}

	_, err := service.RegisterUser(context.Background(), req)
	assert.Error(t, err, "Should return error when CheckEmailExists fails")
}
