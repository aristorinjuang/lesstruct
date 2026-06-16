package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authpkg "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/domain/auth"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestLoginService_Login_Success tests successful login with verified user
func TestLoginService_Login_Success(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockUserRepo.EXPECT().
		UpdateLastLoginAt(mock.Anything, 123).
		Return(nil)

	mockFailedLoginRepo.EXPECT().
		ResetAttempts(mock.Anything, 123).
		Return(nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.NoError(t, err, "Expected successful login")
	require.NotNil(t, result, "Expected login result")

	assert.Equal(t, "testuser", result.Username, "Expected username 'testuser'")
	assert.Equal(t, "Contributor", result.Role, "Expected role 'Contributor'")
}

// TestLoginService_Login_InvalidCredentials tests login with invalid credentials
func TestLoginService_Login_InvalidCredentials(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	attempts := 0

	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockFailedLoginRepo.EXPECT().
		GetByUserID(mock.Anything, 123).
		RunAndReturn(func(ctx context.Context, userID int) (*repository.FailedLoginAttempt, error) {
			assert.Equal(t, 123, userID)
			if attempts == 0 {
				return nil, repository.ErrFailedLoginAttemptNotFound
			}
			return &repository.FailedLoginAttempt{
				ID:            1,
				UserID:        userID,
				Attempts:      attempts,
				LastAttemptAt: time.Now(),
			}, nil
		})

	mockFailedLoginRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, attempt *repository.FailedLoginAttempt) {
			attempts = 1
		}).
		Return(nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act - Use wrong password
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	// Assert
	require.Error(t, err, "Expected error for invalid credentials")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
	assert.Equal(t, 1, attempts, "Expected 1 failed attempt")
}

// TestLoginService_Login_UserNotFound tests login with non-existent user
func TestLoginService_Login_UserNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "nonexistent").
		Return(nil, nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "nonexistent",
		Password: "Password123!",
	})

	// Assert
	require.Error(t, err, "Expected error for non-existent user")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
}

// TestLoginService_Login_EmailNotVerified tests login with pending email verification
func TestLoginService_Login_EmailNotVerified(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz",
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "pending",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.Error(t, err, "Expected error for unverified email")
	assert.ErrorIs(t, err, auth.ErrEmailNotVerified, "Expected ErrEmailNotVerified")
	assert.Nil(t, result, "Expected nil result for failed login")
}

// TestLoginService_Login_AccountLocked tests login with locked account
func TestLoginService_Login_AccountLocked(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	lockedUntil := time.Now().Add(15 * time.Minute)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz",
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(true, &lockedUntil, nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.Error(t, err, "Expected error for locked account")
	assert.ErrorIs(t, err, auth.ErrAccountLocked, "Expected ErrAccountLocked")
	assert.Nil(t, result, "Expected nil result for failed login")
}

// TestLoginService_Login_EmptyUsername tests login with empty username
func TestLoginService_Login_EmptyUsername(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "",
		Password: "Password123!",
	})

	// Assert
	require.Error(t, err, "Expected error for empty username")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
}

// TestLoginService_Login_EmptyPassword tests login with empty password
func TestLoginService_Login_EmptyPassword(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "",
	})

	// Assert
	require.Error(t, err, "Expected error for empty password")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
}

// TestLoginService_Login_AccountLockoutAfterThreeFailedAttempts tests account lockout after 3 failed attempts
func TestLoginService_Login_AccountLockoutAfterThreeFailedAttempts(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	attempts := 0
	wasLocked := false

	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	// Each Login() call: GetUserByUsername + IsLocked
	// Then on wrong password: handleFailedLogin -> GetByUserID, then either Create or IncrementAttempts or LockAccount
	//
	// Attempt 1: GetByUserID -> not found -> Create
	// Attempt 2: GetByUserID -> found (1 attempt) -> IncrementAttempts
	// Attempt 3: GetByUserID -> found (2 attempts) -> newAttemptCount=3 -> LockAccount

	// 3 calls to GetUserByUsername
	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "verified",
		}, nil).
		Times(3)

	// 3 calls to IsLocked (all not locked)
	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil).
		Times(3)

	// GetByUserID called 3 times with different attempt states
	mockFailedLoginRepo.EXPECT().
		GetByUserID(mock.Anything, 123).
		RunAndReturn(func(ctx context.Context, userID int) (*repository.FailedLoginAttempt, error) {
			if attempts == 0 {
				return nil, repository.ErrFailedLoginAttemptNotFound
			}
			return &repository.FailedLoginAttempt{
				ID:            1,
				UserID:        userID,
				Attempts:      attempts,
				LastAttemptAt: time.Now(),
			}, nil
		}).
		Times(3)

	// Create called once (first failed attempt)
	mockFailedLoginRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, attempt *repository.FailedLoginAttempt) {
			attempts = 1
		}).
		Return(nil)

	// IncrementAttempts called once (second failed attempt)
	mockFailedLoginRepo.EXPECT().
		IncrementAttempts(mock.Anything, 123).
		Run(func(ctx context.Context, userID int) {
			attempts++
		}).
		Return(nil)

	// LockAccount called once (third failed attempt)
	mockFailedLoginRepo.EXPECT().
		LockAccount(mock.Anything, 123, mock.Anything).
		Run(func(ctx context.Context, userID int, lockoutDuration time.Duration) {
			wasLocked = true
		}).
		Return(nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act - Perform 3 failed login attempts (using wrong password)
	for i := 1; i <= 3; i++ {
		_, err := service.Login(ctx, auth.LoginRequest{
			Username: "testuser",
			Password: "WrongPassword1!",
		})

		assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "Attempt %d: Expected ErrInvalidCredentials", i)
	}

	// Assert - Verify account was locked
	assert.True(t, wasLocked, "Expected account to be locked after 3 failed attempts")
	assert.GreaterOrEqual(t, attempts, 2, "Expected at least 2 failed attempts before lock")
}

// TestLoginService_Login_ResetFailedAttemptsOnSuccess tests that failed attempts are reset on successful login
func TestLoginService_Login_ResetFailedAttemptsOnSuccess(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	wasReset := false

	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockUserRepo.EXPECT().
		UpdateLastLoginAt(mock.Anything, 123).
		Return(nil)

	mockFailedLoginRepo.EXPECT().
		ResetAttempts(mock.Anything, 123).
		Run(func(ctx context.Context, userID int) {
			wasReset = true
		}).
		Return(nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.NoError(t, err, "Expected successful login")
	require.NotNil(t, result, "Expected login result")
	assert.True(t, wasReset, "Expected failed attempts to be reset on successful login")
}

// TestLoginService_IsAccountLocked tests checking if an account is locked
func TestLoginService_IsAccountLocked(t *testing.T) {
	// Arrange
	ctx := context.Background()
	lockedUntil := time.Now().Add(15 * time.Minute)

	tests := []struct {
		name           string
		userExists     bool
		isLocked       bool
		expectedLocked bool
	}{
		{
			name:           "Account is locked",
			userExists:     true,
			isLocked:       true,
			expectedLocked: true,
		},
		{
			name:           "Account is not locked",
			userExists:     true,
			isLocked:       false,
			expectedLocked: false,
		},
		{
			name:           "User does not exist",
			userExists:     false,
			isLocked:       false,
			expectedLocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := repomocks.NewMockUserRepo(t)
			mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

			if tt.userExists {
				mockUserRepo.EXPECT().
					GetUserByUsername(mock.Anything, "testuser").
					Return(&repository.User{
						ID:       123,
						Username: "testuser",
					}, nil)

				mockFailedLoginRepo.EXPECT().
					IsLocked(mock.Anything, 123).
					Return(tt.isLocked, &lockedUntil, nil)
			} else {
				mockUserRepo.EXPECT().
					GetUserByUsername(mock.Anything, "testuser").
					Return(nil, nil)
			}

			service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

			// Act
			isLocked, lockedUntil, err := service.IsAccountLocked(ctx, "testuser")

			// Assert
			require.NoError(t, err, "Expected no error")
			assert.Equal(t, tt.expectedLocked, isLocked, "Expected locked=%v", tt.expectedLocked)

			if tt.expectedLocked {
				assert.NotNil(t, lockedUntil, "Expected lockedUntil to be set when account is locked")
			}
		})
	}
}

// TestLoginService_GetFailedLoginAttempts tests getting failed login attempt count
func TestLoginService_GetFailedLoginAttempts(t *testing.T) {
	// Arrange
	ctx := context.Background()

	tests := []struct {
		name             string
		userExists       bool
		attemptExists    bool
		expectedAttempts int
	}{
		{
			name:             "User has failed attempts",
			userExists:       true,
			attemptExists:    true,
			expectedAttempts: 2,
		},
		{
			name:             "User has no failed attempts",
			userExists:       true,
			attemptExists:    false,
			expectedAttempts: 0,
		},
		{
			name:             "User does not exist",
			userExists:       false,
			attemptExists:    false,
			expectedAttempts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := repomocks.NewMockUserRepo(t)
			mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

			if tt.userExists {
				mockUserRepo.EXPECT().
					GetUserByUsername(mock.Anything, "testuser").
					Return(&repository.User{
						ID:       123,
						Username: "testuser",
					}, nil)

				if tt.attemptExists {
					mockFailedLoginRepo.EXPECT().
						GetByUserID(mock.Anything, 123).
						Return(&repository.FailedLoginAttempt{
							Attempts: tt.expectedAttempts,
						}, nil)
				} else {
					mockFailedLoginRepo.EXPECT().
						GetByUserID(mock.Anything, 123).
						Return(nil, repository.ErrFailedLoginAttemptNotFound)
				}
			} else {
				mockUserRepo.EXPECT().
					GetUserByUsername(mock.Anything, "testuser").
					Return(nil, nil)
			}

			service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

			// Act
			attempts, err := service.GetFailedLoginAttempts(ctx, "testuser")

			// Assert
			require.NoError(t, err, "Expected no error")
			assert.Equal(t, tt.expectedAttempts, attempts, "Expected %d attempts", tt.expectedAttempts)
		})
	}
}

// TestLoginService_Login_DatabaseError tests database error handling
func TestLoginService_Login_DatabaseError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(nil, errors.New("database connection lost"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "Password123!",
	})

	// Assert
	require.Error(t, err, "Expected error for database failure")
	assert.Nil(t, result, "Expected nil result for database error")
}

// TestLoginService_Login_SuspendedStatus tests login with suspended account
func TestLoginService_Login_SuspendedStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz",
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "suspended",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.Error(t, err, "Expected error for suspended status")
	assert.ErrorIs(t, err, auth.ErrAccountSuspended, "Expected ErrAccountSuspended")
	assert.Nil(t, result, "Expected nil result for suspended status")
}

// TestLoginService_Login_ResetAttemptsError tests successful login when ResetAttempts fails
func TestLoginService_Login_ResetAttemptsError(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockUserRepo.EXPECT().
		UpdateLastLoginAt(mock.Anything, 123).
		Return(nil)

	mockFailedLoginRepo.EXPECT().
		ResetAttempts(mock.Anything, 123).
		Return(errors.New("failed to reset attempts"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act - login should succeed even if ResetAttempts fails
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.NoError(t, err, "Expected successful login despite ResetAttempts error")
	require.NotNil(t, result, "Expected login result")
	assert.Equal(t, "testuser", result.Username, "Expected username 'testuser'")
}

// TestLoginService_Login_IsLockedCheckError tests error when checking lock status
func TestLoginService_Login_IsLockedCheckError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz",
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, errors.New("database error checking lock status"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "Password123!",
	})

	// Assert
	require.Error(t, err, "Expected error when IsLocked fails")
	assert.Nil(t, result, "Expected nil result")
}

// TestLoginService_IsAccountLocked_DatabaseError tests database error in IsAccountLocked
func TestLoginService_IsAccountLocked_DatabaseError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(nil, errors.New("database connection lost"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	isLocked, lockedUntil, err := service.IsAccountLocked(ctx, "testuser")

	// Assert
	require.Error(t, err, "Expected error for database failure")
	assert.False(t, isLocked, "Expected isLocked to be false on error")
	assert.Nil(t, lockedUntil, "Expected lockedUntil to be nil on error")
}

// TestLoginService_IsAccountLocked_FailedLoginRepoError tests error in FailedLoginRepo.IsLocked
func TestLoginService_IsAccountLocked_FailedLoginRepoError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:       123,
			Username: "testuser",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, errors.New("failed to check lock status"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	isLocked, lockedUntil, err := service.IsAccountLocked(ctx, "testuser")

	// Assert
	require.Error(t, err, "Expected error when FailedLoginRepo.IsLocked fails")
	assert.False(t, isLocked, "Expected isLocked to be false on error")
	assert.Nil(t, lockedUntil, "Expected lockedUntil to be nil on error")
}

// TestLoginService_GetFailedLoginAttempts_DatabaseError tests database error in GetUserByUsername
func TestLoginService_GetFailedLoginAttempts_DatabaseError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(nil, errors.New("database connection lost"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	attempts, err := service.GetFailedLoginAttempts(ctx, "testuser")

	// Assert
	require.Error(t, err, "Expected error for database failure")
	assert.Equal(t, 0, attempts, "Expected 0 attempts on error")
}

// TestLoginService_GetFailedLoginAttempts_GetByUserIDError tests error in GetByUserID
func TestLoginService_GetFailedLoginAttempts_GetByUserIDError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:       123,
			Username: "testuser",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		GetByUserID(mock.Anything, 123).
		Return(nil, errors.New("failed to get attempt record"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	attempts, err := service.GetFailedLoginAttempts(ctx, "testuser")

	// Assert
	require.Error(t, err, "Expected error when GetByUserID fails")
	assert.Equal(t, 0, attempts, "Expected 0 attempts on error")
}

// TestLoginService_Login_FailedLoginDatabaseError tests login when failed login recording fails
func TestLoginService_Login_FailedLoginDatabaseError(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockFailedLoginRepo.EXPECT().
		GetByUserID(mock.Anything, 123).
		Return(nil, errors.New("database error getting failed attempts"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act - login with wrong password should fail due to database error
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	// Assert
	require.Error(t, err, "Expected error when failed login recording fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}

// TestLoginService_Login_FailedLoginIncrementError tests login when incrementing failed attempts fails
func TestLoginService_Login_FailedLoginIncrementError(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockFailedLoginRepo.EXPECT().
		GetByUserID(mock.Anything, 123).
		Return(&repository.FailedLoginAttempt{
			Attempts: 1,
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IncrementAttempts(mock.Anything, 123).
		Return(errors.New("database error incrementing attempts"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act - login with wrong password should fail due to database error
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	// Assert
	require.Error(t, err, "Expected error when incrementing failed attempts fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}

// TestLoginService_Login_FirstFailedLoginCreateError tests login when creating first failed attempt fails
func TestLoginService_Login_FirstFailedLoginCreateError(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockFailedLoginRepo.EXPECT().
		GetByUserID(mock.Anything, 123).
		Return(nil, repository.ErrFailedLoginAttemptNotFound)

	mockFailedLoginRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(errors.New("database error creating failed attempt"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act - login with wrong password should fail due to database error
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	// Assert
	require.Error(t, err, "Expected error when creating failed attempt fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}

// TestLoginService_Login_SoftDeletedStatus tests login with soft_deleted account
func TestLoginService_Login_SoftDeletedStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz",
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "soft_deleted",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.Error(t, err, "Expected error for soft_deleted status")
	assert.ErrorIs(t, err, auth.ErrAccountDeleted, "Expected ErrAccountDeleted")
	assert.Nil(t, result, "Expected nil result for soft_deleted status")
}

// TestLoginService_Login_UnknownStatus tests login with an unknown status
func TestLoginService_Login_UnknownStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: "$2a$12$abcdefghijklmnopqrstuvwxyz",
			Email:        "test@example.com",
			Role:         "Contributor",
			Status:       "unknown_status",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	// Assert
	require.Error(t, err, "Expected error for unknown status")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for unknown status")
}

// TestLoginService_Login_LockAccountError tests login when locking account fails on third failed attempt
func TestLoginService_Login_LockAccountError(t *testing.T) {
	// Arrange - generate a real bcrypt hash for "CorrectPassword123!"
	passwordHash, err := authpkg.HashPassword("CorrectPassword123!")
	require.NoError(t, err, "Failed to generate password hash")

	ctx := context.Background()
	mockUserRepo := repomocks.NewMockUserRepo(t)
	mockFailedLoginRepo := repomocks.NewMockFailedLoginAttemptRepo(t)

	mockUserRepo.EXPECT().
		GetUserByUsername(mock.Anything, "testuser").
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: passwordHash,
			Status:       "verified",
		}, nil)

	mockFailedLoginRepo.EXPECT().
		IsLocked(mock.Anything, 123).
		Return(false, nil, nil)

	mockFailedLoginRepo.EXPECT().
		GetByUserID(mock.Anything, 123).
		Return(&repository.FailedLoginAttempt{
			Attempts: 2,
		}, nil)

	mockFailedLoginRepo.EXPECT().
		LockAccount(mock.Anything, 123, mock.Anything).
		Return(errors.New("database error locking account"))

	service := auth.NewLoginService(mockUserRepo, mockFailedLoginRepo, nil)

	// Act - login with wrong password should fail due to database error
	result, err := service.Login(ctx, auth.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	// Assert
	require.Error(t, err, "Expected error when locking account fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}
