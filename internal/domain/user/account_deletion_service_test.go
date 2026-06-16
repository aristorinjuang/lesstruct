package user_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newAccountDeletionService(
	t *testing.T,
	userRepo *repomocks.MockUserRepo,
	deletionRepo *repomocks.MockUserDeletionRepo,
	emailService *emailmocks.MockEmailService,
) *user.AccountDeletionService {
	t.Helper()
	return user.NewAccountDeletionService(
		userRepo,
		deletionRepo,
		emailService,
		util.NewLogger(os.Stdout),
	)
}

func TestAccountDeletionService_DeleteAccount_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:   1,
			Role: "User",
		}, nil)

	deletionRepo.EXPECT().
		DeleteAllUserData(mock.Anything, 1).
		Return(nil)

	emailService.EXPECT().
		SendAccountDeletedNotification(mock.Anything, "test@example.com", "testuser").
		Return(nil)

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 1, "testuser", "test@example.com")
	require.NoError(t, err)
}

func TestAccountDeletionService_DeleteAccount_LastAdmin(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:   1,
			Role: "Admin",
		}, nil)

	deletionRepo.EXPECT().
		CountUsersByRoleAndStatus(mock.Anything, "Admin", "active").
		Return(1, nil)

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 1, "admin", "admin@example.com")
	require.ErrorIs(t, err, user.ErrLastAdminDeletionForbidden)
}

func TestAccountDeletionService_DeleteAccount_AdminWithMultipleAdmins(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:   1,
			Role: "Admin",
		}, nil)

	deletionRepo.EXPECT().
		CountUsersByRoleAndStatus(mock.Anything, "Admin", "active").
		Return(2, nil)

	deletionRepo.EXPECT().
		DeleteAllUserData(mock.Anything, 1).
		Return(nil)

	emailService.EXPECT().
		SendAccountDeletedNotification(
			mock.Anything,
			"admin@example.com",
			"admin",
		).
		Return(nil)

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 1, "admin", "admin@example.com")
	require.NoError(t, err)
}

func TestAccountDeletionService_DeleteAccount_NonAdminUser(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:   1,
			Role: "User",
		}, nil)

	deletionRepo.EXPECT().
		DeleteAllUserData(mock.Anything, 1).
		Return(nil)

	emailService.EXPECT().
		SendAccountDeletedNotification(
			mock.Anything,
			"test@example.com",
			"testuser",
		).
		Return(nil)

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 1, "testuser", "test@example.com")
	require.NoError(t, err)
}

func TestAccountDeletionService_DeleteAccount_UserNotFound(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 999).
		Return(nil, errors.New("user not found"))

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 999, "unknown", "unknown@example.com")
	require.Error(t, err)
}

func TestAccountDeletionService_DeleteAccount_DeleteAllUserDataError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:   1,
			Role: "User",
		}, nil)

	deletionRepo.EXPECT().
		DeleteAllUserData(mock.Anything, 1).
		Return(errors.New("database error"))

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 1, "testuser", "test@example.com")
	require.Error(t, err)
}

func TestAccountDeletionService_DeleteAccount_EmailError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:   1,
			Role: "User",
		}, nil)

	deletionRepo.EXPECT().
		DeleteAllUserData(mock.Anything, 1).
		Return(nil)

	// Email error should not fail the deletion (log but continue)
	emailService.EXPECT().
		SendAccountDeletedNotification(
			mock.Anything,
			"test@example.com",
			"testuser",
		).
		Return(errors.New("email service error"))

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 1, "testuser", "test@example.com")
	require.NoError(t, err)
}

func TestAccountDeletionService_DeleteAccount_CountAdminsError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	deletionRepo := repomocks.NewMockUserDeletionRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID:   1,
			Role: "Admin",
		}, nil)

	deletionRepo.EXPECT().
		CountUsersByRoleAndStatus(mock.Anything, "Admin", "active").
		Return(0, errors.New("database error counting admins"))

	service := newAccountDeletionService(t, userRepo, deletionRepo, emailService)

	err := service.DeleteAccount(context.Background(), 1, "admin", "admin@example.com")
	require.Error(t, err)
}

func TestAccountDeletionService_ValidateConfirmationString(t *testing.T) {
	service := user.NewAccountDeletionService(nil, nil, nil, util.NewLogger(os.Stdout))

	tests := []struct {
		name          string
		confirmation  string
		expectError   bool
		expectedError error
	}{
		{
			name:         "valid confirmation",
			confirmation: "DELETE",
			expectError:  false,
		},
		{
			name:          "invalid lowercase",
			confirmation:  "delete",
			expectError:   true,
			expectedError: user.ErrInvalidConfirmationString,
		},
		{
			name:          "invalid with leading space",
			confirmation:  " DELETE",
			expectError:   true,
			expectedError: user.ErrInvalidConfirmationString,
		},
		{
			name:          "invalid with trailing space",
			confirmation:  "DELETE ",
			expectError:   true,
			expectedError: user.ErrInvalidConfirmationString,
		},
		{
			name:          "empty string",
			confirmation:  "",
			expectError:   true,
			expectedError: user.ErrInvalidConfirmationString,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateConfirmationString(tt.confirmation)
			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
