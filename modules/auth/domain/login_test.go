package auth_test

import (
	"context"
	"errors"
	"testing"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func verifiedUser() *userdomain.User {
	return &userdomain.User{
		ID:           123,
		Username:     "testuser",
		PasswordHash: "hashed:CorrectPassword123!",
		Email:        "test@example.com",
		Role:         "Contributor",
		Status:       "verified",
	}
}

func TestLoginService_Login_Success(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.NoError(t, err, "Expected successful login")
	require.NotNil(t, result, "Expected login result")

	assert.Equal(t, "testuser", result.Username, "Expected username 'testuser'")
	assert.Equal(t, "Contributor", result.Role, "Expected role 'Contributor'")
	assert.True(t, failed.wasReset, "Expected failed attempts to reset on success")
}

func TestLoginService_Login_InvalidCredentials(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	require.Error(t, err, "Expected error for invalid credentials")
	assert.ErrorIs(t, err, authdomain.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
	assert.Equal(t, 1, failed.attempts[123], "Expected 1 failed attempt")
}

func TestLoginService_Login_UserNotFound(t *testing.T) {
	store := &stubUserStore{}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "nonexistent",
		Password: "Password123!",
	})

	require.Error(t, err, "Expected error for non-existent user")
	assert.ErrorIs(t, err, authdomain.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
}

func TestLoginService_Login_EmailNotVerified(t *testing.T) {
	u := verifiedUser()
	u.Status = "pending"
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": u}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.Error(t, err, "Expected error for unverified email")
	assert.ErrorIs(t, err, authdomain.ErrEmailNotVerified, "Expected ErrEmailNotVerified")
	assert.Nil(t, result, "Expected nil result for failed login")
}

func TestLoginService_Login_AccountLocked(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{locked: map[int]bool{123: true}}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.Error(t, err, "Expected error for locked account")
	assert.ErrorIs(t, err, authdomain.ErrAccountLocked, "Expected ErrAccountLocked")
	assert.Nil(t, result, "Expected nil result for failed login")
}

func TestLoginService_Login_EmptyUsername(t *testing.T) {
	store := &stubUserStore{}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "",
		Password: "Password123!",
	})

	require.Error(t, err, "Expected error for empty username")
	assert.ErrorIs(t, err, authdomain.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
}

func TestLoginService_Login_EmptyPassword(t *testing.T) {
	store := &stubUserStore{}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "",
	})

	require.Error(t, err, "Expected error for empty password")
	assert.ErrorIs(t, err, authdomain.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for failed login")
}

func TestLoginService_Login_AccountLockoutAfterThreeFailedAttempts(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	for i := 1; i <= 3; i++ {
		_, err := service.Login(context.Background(), authdomain.LoginRequest{
			Username: "testuser",
			Password: "WrongPassword1!",
		})

		assert.ErrorIs(t, err, authdomain.ErrInvalidCredentials, "Attempt %d: Expected ErrInvalidCredentials", i)
	}

	assert.True(t, failed.wasLocked, "Expected account to be locked after 3 failed attempts")
	assert.GreaterOrEqual(t, failed.attempts[123], 2, "Expected at least 2 failed attempts before lock")
}

func TestLoginService_Login_ResetFailedAttemptsOnSuccess(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.NoError(t, err, "Expected successful login")
	require.NotNil(t, result, "Expected login result")
	assert.True(t, failed.wasReset, "Expected failed attempts to be reset on successful login")
}

func TestLoginService_IsAccountLocked(t *testing.T) {
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
			store := &stubUserStore{}
			if tt.userExists {
				store.byUsername = map[string]*userdomain.User{
					"testuser": {ID: 123, Username: "testuser"},
				}
			}
			failed := &stubFailedStore{}
			if tt.isLocked {
				failed.locked = map[int]bool{123: true}
			}

			service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

			isLocked, lockedUntil, err := service.IsAccountLocked(context.Background(), "testuser")

			require.NoError(t, err, "Expected no error")
			assert.Equal(t, tt.expectedLocked, isLocked, "Expected locked=%v", tt.expectedLocked)

			if tt.expectedLocked {
				assert.NotNil(t, lockedUntil, "Expected lockedUntil to be set when account is locked")
			}
		})
	}
}

func TestLoginService_GetFailedLoginAttempts(t *testing.T) {
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
			store := &stubUserStore{}
			if tt.userExists {
				store.byUsername = map[string]*userdomain.User{
					"testuser": {ID: 123, Username: "testuser"},
				}
			}
			failed := &stubFailedStore{}
			if tt.attemptExists {
				failed.attempts = map[int]int{123: tt.expectedAttempts}
			}

			service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

			attempts, err := service.GetFailedLoginAttempts(context.Background(), "testuser")

			require.NoError(t, err, "Expected no error")
			assert.Equal(t, tt.expectedAttempts, attempts, "Expected %d attempts", tt.expectedAttempts)
		})
	}
}

func TestLoginService_Login_DatabaseError(t *testing.T) {
	store := &stubUserStore{getErr: errors.New("database connection lost")}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "Password123!",
	})

	require.Error(t, err, "Expected error for database failure")
	assert.Nil(t, result, "Expected nil result for database error")
}

func TestLoginService_Login_SuspendedStatus(t *testing.T) {
	u := verifiedUser()
	u.Status = "suspended"
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": u}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.Error(t, err, "Expected error for suspended status")
	assert.ErrorIs(t, err, authdomain.ErrAccountSuspended, "Expected ErrAccountSuspended")
	assert.Nil(t, result, "Expected nil result for suspended status")
}

func TestLoginService_Login_ResetAttemptsError(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{resetErr: errors.New("failed to reset attempts")}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	// Act - login should succeed even if ResetAttempts fails
	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.NoError(t, err, "Expected successful login despite ResetAttempts error")
	require.NotNil(t, result, "Expected login result")
	assert.Equal(t, "testuser", result.Username, "Expected username 'testuser'")
}

func TestLoginService_Login_IsLockedCheckError(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{isLockedErr: errors.New("database error checking lock status")}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "Password123!",
	})

	require.Error(t, err, "Expected error when IsLocked fails")
	assert.Nil(t, result, "Expected nil result")
}

func TestLoginService_IsAccountLocked_DatabaseError(t *testing.T) {
	store := &stubUserStore{getErr: errors.New("database connection lost")}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	isLocked, lockedUntil, err := service.IsAccountLocked(context.Background(), "testuser")

	require.Error(t, err, "Expected error for database failure")
	assert.False(t, isLocked, "Expected isLocked to be false on error")
	assert.Nil(t, lockedUntil, "Expected lockedUntil to be nil on error")
}

func TestLoginService_IsAccountLocked_FailedLoginRepoError(t *testing.T) {
	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{
			"testuser": {ID: 123, Username: "testuser"},
		},
	}
	failed := &stubFailedStore{isLockedErr: errors.New("failed to check lock status")}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	isLocked, lockedUntil, err := service.IsAccountLocked(context.Background(), "testuser")

	require.Error(t, err, "Expected error when FailedLoginRepo.IsLocked fails")
	assert.False(t, isLocked, "Expected isLocked to be false on error")
	assert.Nil(t, lockedUntil, "Expected lockedUntil to be nil on error")
}

func TestLoginService_GetFailedLoginAttempts_DatabaseError(t *testing.T) {
	store := &stubUserStore{getErr: errors.New("database connection lost")}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	attempts, err := service.GetFailedLoginAttempts(context.Background(), "testuser")

	require.Error(t, err, "Expected error for database failure")
	assert.Equal(t, 0, attempts, "Expected 0 attempts on error")
}

func TestLoginService_GetFailedLoginAttempts_GetByUserIDError(t *testing.T) {
	store := &stubUserStore{
		byUsername: map[string]*userdomain.User{
			"testuser": {ID: 123, Username: "testuser"},
		},
	}
	failed := &stubFailedStore{getErr: errors.New("failed to get attempt record")}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	attempts, err := service.GetFailedLoginAttempts(context.Background(), "testuser")

	require.Error(t, err, "Expected error when GetByUserID fails")
	assert.Equal(t, 0, attempts, "Expected 0 attempts on error")
}

func TestLoginService_Login_FailedLoginDatabaseError(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{getErr: errors.New("database error getting failed attempts")}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	require.Error(t, err, "Expected error when failed login recording fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}

func TestLoginService_Login_FailedLoginIncrementError(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{
		attempts:     map[int]int{123: 1},
		incrementErr: errors.New("database error incrementing attempts"),
	}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	require.Error(t, err, "Expected error when incrementing failed attempts fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}

func TestLoginService_Login_FirstFailedLoginCreateError(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{createErr: errors.New("database error creating failed attempt")}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	require.Error(t, err, "Expected error when creating failed attempt fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}

func TestLoginService_Login_SoftDeletedStatus(t *testing.T) {
	u := verifiedUser()
	u.Status = "soft_deleted"
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": u}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.Error(t, err, "Expected error for soft_deleted status")
	assert.ErrorIs(t, err, authdomain.ErrAccountDeleted, "Expected ErrAccountDeleted")
	assert.Nil(t, result, "Expected nil result for soft_deleted status")
}

func TestLoginService_Login_UnknownStatus(t *testing.T) {
	u := verifiedUser()
	u.Status = "unknown_status"
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": u}}
	failed := &stubFailedStore{}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "CorrectPassword123!",
	})

	require.Error(t, err, "Expected error for unknown status")
	assert.ErrorIs(t, err, authdomain.ErrInvalidCredentials, "Expected ErrInvalidCredentials")
	assert.Nil(t, result, "Expected nil result for unknown status")
}

func TestLoginService_Login_LockAccountError(t *testing.T) {
	store := &stubUserStore{byUsername: map[string]*userdomain.User{"testuser": verifiedUser()}}
	failed := &stubFailedStore{
		attempts: map[int]int{123: 2},
		lockErr:  errors.New("database error locking account"),
	}

	service := authdomain.NewLoginService(store, failed, stubPolicy{}, nil)

	result, err := service.Login(context.Background(), authdomain.LoginRequest{
		Username: "testuser",
		Password: "WrongPassword1!",
	})

	require.Error(t, err, "Expected error when locking account fails")
	assert.Contains(t, err.Error(), "failed to record failed attempt", "Expected error about recording failed attempt")
	assert.Nil(t, result, "Expected nil result")
}
