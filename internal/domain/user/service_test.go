package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	"github.com/aristorinjuang/lesstruct/internal/domain/user/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestUserManagementService_GetPendingUsers_Success tests successful retrieval of pending users
func TestUserManagementService_GetPendingUsers_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetPendingUsers(mock.Anything, 100, 0).
		Return([]*repository.User{
			{ID: 1, Username: "user1", Email: "user1@example.com", Status: "pending"},
			{ID: 2, Username: "user2", Email: "user2@example.com", Status: "pending"},
		}, nil)

	// Act
	result, err := service.GetPendingUsers(context.Background(), 100, 0)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "user1", result[0].Username)
}

// TestUserManagementService_GetPendingUsers_Empty tests retrieval when no pending users exist
func TestUserManagementService_GetPendingUsers_Empty(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetPendingUsers(mock.Anything, 100, 0).
		Return([]*repository.User{}, nil)

	// Act
	result, err := service.GetPendingUsers(context.Background(), 100, 0)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestUserManagementService_GetPendingUsers_RepositoryError tests error handling when repository fails
func TestUserManagementService_GetPendingUsers_RepositoryError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetPendingUsers(mock.Anything, 100, 0).
		Return(nil, errors.New("repository error"))

	// Act
	_, err := service.GetPendingUsers(context.Background(), 100, 0)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_ApproveUser_Success tests successful user approval
func TestUserManagementService_ApproveUser_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 1, "pending", "verified").
		Return(nil)

	// Act
	err := service.ApproveUser(context.Background(), 1)

	// Assert
	require.NoError(t, err)
}

// TestUserManagementService_ApproveUser_UserNotFound tests approval of non-existent user
func TestUserManagementService_ApproveUser_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 999, "pending", "verified").
		Return(errors.New("user not found with ID 999 or not in pending status"))

	// Act
	err := service.ApproveUser(context.Background(), 999)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_ApproveUser_UpdateError tests error handling when status update fails
func TestUserManagementService_ApproveUser_UpdateError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 1, "pending", "verified").
		Return(errors.New("database error"))

	// Act
	err := service.ApproveUser(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_ApproveUser_NonPendingStatus tests approving a user that is not pending
func TestUserManagementService_ApproveUser_NonPendingStatus(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 1, "pending", "verified").
		Return(errors.New("user not found with ID 1 or not in pending status"))

	// Act
	err := service.ApproveUser(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_RejectUser_Success tests successful user rejection
func TestUserManagementService_RejectUser_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "testuser", Email: "test@example.com", Status: "pending"}, nil)

	mockUserRepo.EXPECT().
		DeleteUser(mock.Anything, 1).
		Return(nil)

	// Act
	err := service.RejectUser(context.Background(), 1)

	// Assert
	require.NoError(t, err)
}

// TestUserManagementService_RejectUser_UserNotFound tests rejection of non-existent user
func TestUserManagementService_RejectUser_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 999).
		Return(nil, errors.New("user not found"))

	// Act
	err := service.RejectUser(context.Background(), 999)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

// TestUserManagementService_RejectUser_GetUserError tests error handling when GetUserByID returns a different error
func TestUserManagementService_RejectUser_GetUserError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(nil, errors.New("database error"))

	// Act
	err := service.RejectUser(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_RejectUser_DeleteError tests error handling when delete fails
func TestUserManagementService_RejectUser_DeleteError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "testuser", Email: "test@example.com", Status: "pending"}, nil)

	mockUserRepo.EXPECT().
		DeleteUser(mock.Anything, 1).
		Return(errors.New("delete failed"))

	// Act
	err := service.RejectUser(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_MarkUserAsSpam_Success tests successful marking user as spam
func TestUserManagementService_MarkUserAsSpam_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "spammer", Email: "spam@example.com", Status: "pending"}, nil)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(false, nil)

	mockBlockedEmailRepo.EXPECT().
		BlockEmail(mock.Anything, "spam@example.com", "marked_as_spam").
		Return(nil)

	mockUserRepo.EXPECT().
		DeleteUser(mock.Anything, 1).
		Return(nil)

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.NoError(t, err)
}

// TestUserManagementService_MarkUserAsSpam_UserNotFound tests marking non-existent user as spam
func TestUserManagementService_MarkUserAsSpam_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 999).
		Return(nil, errors.New("user not found"))

	// Act
	err := service.MarkUserAsSpam(context.Background(), 999)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

// TestUserManagementService_MarkUserAsSpam_GetUserError tests error handling when GetUserByID returns a different error
func TestUserManagementService_MarkUserAsSpam_GetUserError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(nil, errors.New("database error"))

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_MarkUserAsSpam_IsEmailBlockedError tests error handling when IsEmailBlocked fails
func TestUserManagementService_MarkUserAsSpam_IsEmailBlockedError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "spammer", Email: "spam@example.com", Status: "pending"}, nil)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(false, errors.New("repository error"))

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_MarkUserAsSpam_EmailAlreadyBlocked tests marking user as spam when email is already blocked
func TestUserManagementService_MarkUserAsSpam_EmailAlreadyBlocked(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "spammer", Email: "spam@example.com", Status: "pending"}, nil)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(true, nil)

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.Error(t, err)
	assert.Equal(t, user.ErrEmailAlreadyBlocked, err)
}

// TestUserManagementService_MarkUserAsSpam_DeleteError tests error handling when delete fails
func TestUserManagementService_MarkUserAsSpam_DeleteError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "spammer", Email: "spam@example.com", Status: "pending"}, nil)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(false, nil)

	mockBlockedEmailRepo.EXPECT().
		BlockEmail(mock.Anything, "spam@example.com", "marked_as_spam").
		Return(nil)

	mockUserRepo.EXPECT().
		DeleteUser(mock.Anything, 1).
		Return(errors.New("delete failed"))

	mockBlockedEmailRepo.EXPECT().
		UnblockEmail(mock.Anything, "spam@example.com").
		Return(nil)

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_MarkUserAsSpam_DeleteError_UnblockCompensation tests compensation when delete fails and unblock succeeds
func TestUserManagementService_MarkUserAsSpam_DeleteError_UnblockCompensation(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "spammer", Email: "spam@example.com", Status: "pending"}, nil)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(false, nil)

	mockBlockedEmailRepo.EXPECT().
		BlockEmail(mock.Anything, "spam@example.com", "marked_as_spam").
		Return(nil)

	mockUserRepo.EXPECT().
		DeleteUser(mock.Anything, 1).
		Return(errors.New("delete failed"))

	mockBlockedEmailRepo.EXPECT().
		UnblockEmail(mock.Anything, "spam@example.com").
		Return(nil)

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_MarkUserAsSpam_DeleteAndUnblockBothFail tests when both delete and compensation unblock fail
func TestUserManagementService_MarkUserAsSpam_DeleteAndUnblockBothFail(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "spammer", Email: "spam@example.com", Status: "pending"}, nil)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(false, nil)

	mockBlockedEmailRepo.EXPECT().
		BlockEmail(mock.Anything, "spam@example.com", "marked_as_spam").
		Return(nil)

	mockUserRepo.EXPECT().
		DeleteUser(mock.Anything, 1).
		Return(errors.New("delete failed"))

	mockBlockedEmailRepo.EXPECT().
		UnblockEmail(mock.Anything, "spam@example.com").
		Return(errors.New("unblock failed"))

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.Error(t, err)
	assert.NotEmpty(t, err.Error())
}

// TestUserManagementService_MarkUserAsSpam_BlockEmailError tests error handling when blocking email fails
func TestUserManagementService_MarkUserAsSpam_BlockEmailError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{ID: 1, Username: "spammer", Email: "spam@example.com", Status: "pending"}, nil)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "spam@example.com").
		Return(false, nil)

	mockBlockedEmailRepo.EXPECT().
		BlockEmail(mock.Anything, "spam@example.com", "marked_as_spam").
		Return(errors.New("block failed"))

	// Act
	err := service.MarkUserAsSpam(context.Background(), 1)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_IsEmailBlocked_Blocked tests checking a blocked email
func TestUserManagementService_IsEmailBlocked_Blocked(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "blocked@example.com").
		Return(true, nil)

	// Act
	blocked, err := service.IsEmailBlocked(context.Background(), "blocked@example.com")

	// Assert
	require.NoError(t, err)
	assert.True(t, blocked)
}

// TestUserManagementService_IsEmailBlocked_NotBlocked tests checking a non-blocked email
func TestUserManagementService_IsEmailBlocked_NotBlocked(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "normal@example.com").
		Return(false, nil)

	// Act
	blocked, err := service.IsEmailBlocked(context.Background(), "normal@example.com")

	// Assert
	require.NoError(t, err)
	assert.False(t, blocked)
}

// TestUserManagementService_IsEmailBlocked_RepositoryError tests error handling when repository fails
func TestUserManagementService_IsEmailBlocked_RepositoryError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, mockBlockedEmailRepo)

	mockBlockedEmailRepo.EXPECT().
		IsEmailBlocked(mock.Anything, "test@example.com").
		Return(false, errors.New("repository error"))

	// Act
	_, err := service.IsEmailBlocked(context.Background(), "test@example.com")

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_SuspendUser_Success tests successful user suspension
func TestUserManagementService_SuspendUser_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 123, "verified", "suspended").
		Return(nil)

	// Act
	err := service.SuspendUser(context.Background(), 123)

	// Assert
	require.NoError(t, err)
}

// TestUserManagementService_SuspendUser_Failure tests user suspension failure
func TestUserManagementService_SuspendUser_Failure(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 123, "verified", "suspended").
		Return(errors.New("user not found"))

	// Act
	err := service.SuspendUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_SuspendUser_AlreadySuspended tests suspending an already suspended user
func TestUserManagementService_SuspendUser_AlreadySuspended(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("suspended", nil)

	// Act
	err := service.SuspendUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrInvalidStatus))
}

// TestUserManagementService_SuspendUser_SoftDeleted tests suspending a soft-deleted user
func TestUserManagementService_SuspendUser_SoftDeleted(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("soft_deleted", nil)

	// Act
	err := service.SuspendUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrInvalidStatus))
}

// TestUserManagementService_SuspendUser_GetStatusError tests error propagation when GetUserStatus fails
func TestUserManagementService_SuspendUser_GetStatusError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("", errors.New("database error"))

	// Act
	err := service.SuspendUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_UnsuspendUser_Success tests successful user unsuspension
func TestUserManagementService_UnsuspendUser_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("suspended", nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 123, "suspended", "verified").
		Return(nil)

	// Act
	err := service.UnsuspendUser(context.Background(), 123)

	// Assert
	require.NoError(t, err)
}

// TestUserManagementService_UnsuspendUser_Failure tests user unsuspension failure
func TestUserManagementService_UnsuspendUser_Failure(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("suspended", nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 123, "suspended", "verified").
		Return(errors.New("user not found"))

	// Act
	err := service.UnsuspendUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_UnsuspendUser_NotSuspended tests unsuspending a user that is not suspended
func TestUserManagementService_UnsuspendUser_NotSuspended(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)

	// Act
	err := service.UnsuspendUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrInvalidStatus))
}

// TestUserManagementService_UnsuspendUser_GetStatusError tests error propagation when GetUserStatus fails
func TestUserManagementService_UnsuspendUser_GetStatusError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("", errors.New("database error"))

	// Act
	err := service.UnsuspendUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_SoftDeleteUser_Success tests successful user soft deletion
func TestUserManagementService_SoftDeleteUser_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 123, "verified", "soft_deleted").
		Return(nil)

	// Act
	err := service.SoftDeleteUser(context.Background(), 123)

	// Assert
	require.NoError(t, err)
}

// TestUserManagementService_SoftDeleteUser_Failure tests user soft deletion failure
func TestUserManagementService_SoftDeleteUser_Failure(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)

	mockUserRepo.EXPECT().
		UpdateUserStatusIfCurrentStatus(mock.Anything, 123, "verified", "soft_deleted").
		Return(errors.New("user not found"))

	// Act
	err := service.SoftDeleteUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_SoftDeleteUser_AlreadySoftDeleted tests soft deleting an already soft-deleted user
func TestUserManagementService_SoftDeleteUser_AlreadySoftDeleted(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("soft_deleted", nil)

	// Act
	err := service.SoftDeleteUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrInvalidStatus))
}

// TestUserManagementService_SoftDeleteUser_GetStatusError tests error propagation when GetUserStatus fails
func TestUserManagementService_SoftDeleteUser_GetStatusError(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("", errors.New("database error"))

	// Act
	err := service.SoftDeleteUser(context.Background(), 123)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_GetAllUsers_Success tests successful retrieval of all users
func TestUserManagementService_GetAllUsers_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetAllUsers(mock.Anything, "", 100, 0).
		Return([]*repository.User{
			{ID: 1, Username: "user1", Email: "user1@example.com", Status: "verified"},
			{ID: 2, Username: "user2", Email: "user2@example.com", Status: "suspended"},
		}, nil)

	// Act
	result, err := service.GetAllUsers(context.Background(), "", 100, 0)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 2)
}

// TestUserManagementService_GetAllUsers_WithFilter tests retrieval with status filter
func TestUserManagementService_GetAllUsers_WithFilter(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetAllUsers(mock.Anything, "verified", 100, 0).
		Return([]*repository.User{
			{ID: 1, Username: "user1", Email: "user1@example.com", Status: "verified"},
		}, nil)

	// Act
	result, err := service.GetAllUsers(context.Background(), "verified", 100, 0)

	// Assert
	require.NoError(t, err)
	require.Len(t, result, 1)
}

// TestUserManagementService_GetAllUsers_Failure tests retrieval failure
func TestUserManagementService_GetAllUsers_Failure(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetAllUsers(mock.Anything, "", 100, 0).
		Return(nil, errors.New("database error"))

	// Act
	_, err := service.GetAllUsers(context.Background(), "", 100, 0)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_GetUserStatus_Success tests successful retrieval of user status
func TestUserManagementService_GetUserStatus_Success(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("verified", nil)

	// Act
	status, err := service.GetUserStatus(context.Background(), 123)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "verified", status)
}

// TestUserManagementService_GetUserStatus_Failure tests user status retrieval failure
func TestUserManagementService_GetUserStatus_Failure(t *testing.T) {
	// Arrange
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserStatus(mock.Anything, 123).
		Return("", errors.New("user not found"))

	// Act
	_, err := service.GetUserStatus(context.Background(), 123)

	// Assert
	require.Error(t, err)
}

// TestUserManagementService_UpdateUserProfile_Success tests successful full profile update
func TestUserManagementService_UpdateUserProfile_Success(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Old Name", Role: "Commentator",
		}, nil).Once()

	mockUserRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "new@example.com").
		Return(false, nil)

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "New Name", "new@example.com", "Contributor", mock.Anything).
		Return(nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "new@example.com",
			Name: "New Name", Role: "Contributor", Status: "verified", CreatedAt: "2025-01-01",
		}, nil).Once()

	updated, err := service.UpdateUserProfile(context.Background(), 1, "New Name", "new@example.com", "Contributor", nil)

	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "new@example.com", updated.Email)
	assert.Equal(t, "Contributor", updated.Role)
}

// TestUserManagementService_UpdateUserProfile_PartialNameOnly tests updating only the name
func TestUserManagementService_UpdateUserProfile_PartialNameOnly(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Old Name", Role: "Admin",
		}, nil).Once()

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "Only Name", "test@example.com", "Admin", mock.Anything).
		Return(nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Only Name", Role: "Admin",
		}, nil).Once()

	updated, err := service.UpdateUserProfile(context.Background(), 1, "Only Name", "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, "Only Name", updated.Name)
	assert.Equal(t, "test@example.com", updated.Email)
}

// TestUserManagementService_UpdateUserProfile_EmptyUpdate tests empty update succeeds
func TestUserManagementService_UpdateUserProfile_EmptyUpdate(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Admin",
		}, nil).Once()

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "testuser", "test@example.com", "Admin", mock.Anything).
		Return(nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "testuser", Role: "Admin",
		}, nil).Once()

	_, err := service.UpdateUserProfile(context.Background(), 1, "", "", "", nil)

	require.NoError(t, err)
}

// TestUserManagementService_UpdateUserProfile_UserNotFound tests updating non-existent user
func TestUserManagementService_UpdateUserProfile_UserNotFound(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 999).
		Return(nil, errors.New("user not found"))

	_, err := service.UpdateUserProfile(context.Background(), 999, "Name", "a@b.com", "Admin", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

// TestUserManagementService_UpdateUserProfile_InvalidEmail tests invalid email format
func TestUserManagementService_UpdateUserProfile_InvalidEmail(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	_, err := service.UpdateUserProfile(context.Background(), 1, "", "not-an-email", "", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrEmailInvalid))
}

// TestUserManagementService_UpdateUserProfile_InvalidRole tests invalid role
func TestUserManagementService_UpdateUserProfile_InvalidRole(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	_, err := service.UpdateUserProfile(context.Background(), 1, "", "", "SuperAdmin", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrInvalidRole))
}

// TestUserManagementService_UpdateUserProfile_EmailExists tests email already in use by another user
func TestUserManagementService_UpdateUserProfile_EmailExists(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	mockUserRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "taken@example.com").
		Return(true, nil)

	_, err := service.UpdateUserProfile(context.Background(), 1, "", "taken@example.com", "", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrEmailExists))
}

// TestUserManagementService_UpdateUserProfile_SameEmail tests keeping same email skips uniqueness check
func TestUserManagementService_UpdateUserProfile_SameEmail(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "Test", Role: "Admin",
		}, nil).Once()

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "New Name", "test@example.com", "Admin", mock.Anything).
		Return(nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "test@example.com",
			Name: "New Name", Role: "Admin",
		}, nil).Once()

	_, err := service.UpdateUserProfile(context.Background(), 1, "New Name", "test@example.com", "", nil)

	require.NoError(t, err)
}

// TestUserManagementService_UpdateUserProfile_RepoErrorGetUser tests repo error on GetUserByID
func TestUserManagementService_UpdateUserProfile_RepoErrorGetUser(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(nil, errors.New("database error"))

	_, err := service.UpdateUserProfile(context.Background(), 1, "Name", "a@b.com", "Admin", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

// TestUserManagementService_UpdateUserProfile_RepoErrorCheckEmail tests repo error on CheckEmailExistsForOtherUser
func TestUserManagementService_UpdateUserProfile_RepoErrorCheckEmail(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	mockUserRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "new@example.com").
		Return(false, errors.New("database error"))

	_, err := service.UpdateUserProfile(context.Background(), 1, "", "new@example.com", "", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check email uniqueness")
}

// TestUserManagementService_UpdateUserProfile_RepoErrorUpdateProfile tests repo error on UpdateProfile
func TestUserManagementService_UpdateUserProfile_RepoErrorUpdateProfile(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Test", Role: "Admin",
		}, nil)

	mockUserRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "new@example.com").
		Return(false, nil)

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "testuser", "new@example.com", "Admin", mock.Anything).
		Return(errors.New("database error"))

	_, err := service.UpdateUserProfile(context.Background(), 1, "", "new@example.com", "", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update profile")
}

// TestUserManagementService_UpdateUserProfile_RepoErrorFetchUpdatedUser tests repo error on second GetUserByID
func TestUserManagementService_UpdateUserProfile_RepoErrorFetchUpdatedUser(t *testing.T) {
	mockUserRepo := mocks.NewMockUserRepo(t)
	_ = mocks.NewMockBlockedEmailRepo(t)
	service := user.NewUserManagementService(mockUserRepo, nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(&repository.User{
			ID: 1, Username: "testuser", Email: "old@example.com",
			Name: "Test", Role: "Admin",
		}, nil).Once()

	mockUserRepo.EXPECT().
		CheckEmailExistsForOtherUser(mock.Anything, 1, "new@example.com").
		Return(false, nil)

	mockUserRepo.EXPECT().
		UpdateProfile(mock.Anything, 1, "testuser", "new@example.com", "Admin", mock.Anything).
		Return(nil)

	mockUserRepo.EXPECT().
		GetUserByID(mock.Anything, 1).
		Return(nil, errors.New("database error")).Once()

	_, err := service.UpdateUserProfile(context.Background(), 1, "", "new@example.com", "", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch updated user")
}
