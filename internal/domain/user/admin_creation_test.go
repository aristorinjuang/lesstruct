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

func TestAdminCreateUserService_CreateUser_Success(t *testing.T) {
	mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

	mockUserRepo.EXPECT().CheckUsernameExists(context.Background(), "testuser").Return(false, nil)
	mockUserRepo.EXPECT().CheckEmailExists(context.Background(), "test@example.com").Return(false, nil)
	mockBlockedEmailRepo.EXPECT().IsEmailBlocked(context.Background(), "test@example.com").Return(false, nil)
	mockUserRepo.EXPECT().CreateUser(
		context.Background(),
		mock.AnythingOfType("*repository.User"),
	).RunAndReturn(func(ctx context.Context, u *repository.User) error {
		u.ID = 42
		return nil
	})

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo)
	result, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 42, result.User.ID)
	assert.Equal(t, "testuser", result.User.Username)
	assert.Equal(t, "test@example.com", result.User.Email)
	assert.Equal(t, "Contributor", result.User.Role)
	assert.Equal(t, "verified", result.User.Status)
	assert.NotEmpty(t, result.PlainPassword)
	assert.Len(t, result.PlainPassword, 16)
}

func TestAdminCreateUserService_CreateUser_InvalidUsername(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "invalid user!",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrUsernameInvalid)
}

func TestAdminCreateUserService_CreateUser_InvalidEmail(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "not-an-email",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrEmailInvalid)
}

func TestAdminCreateUserService_CreateUser_InvalidRole(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "SuperAdmin",
	})

	assert.ErrorIs(t, err, user.ErrInvalidRole)
}

func TestAdminCreateUserService_CreateUser_EmptyUsername(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrUsernameInvalid)
}

func TestAdminCreateUserService_CreateUser_EmptyEmail(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrEmailInvalid)
}

func TestAdminCreateUserService_CreateUser_EmptyRole(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "",
	})

	assert.ErrorIs(t, err, user.ErrInvalidRole)
}

func TestAdminCreateUserService_CreateUser_DuplicateUsername(t *testing.T) {
	mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

	mockBlockedEmailRepo.EXPECT().IsEmailBlocked(context.Background(), "test@example.com").Return(false, nil)
	mockUserRepo.EXPECT().CheckUsernameExists(context.Background(), "existinguser").Return(true, nil)

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "existinguser",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrUsernameExists)
}

func TestAdminCreateUserService_CreateUser_DuplicateEmail(t *testing.T) {
	mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

	mockBlockedEmailRepo.EXPECT().IsEmailBlocked(context.Background(), "existing@example.com").Return(false, nil)
	mockUserRepo.EXPECT().CheckUsernameExists(context.Background(), "testuser").Return(false, nil)
	mockUserRepo.EXPECT().CheckEmailExists(context.Background(), "existing@example.com").Return(true, nil)

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "existing@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrEmailExists)
}

func TestAdminCreateUserService_CreateUser_BlockedEmail(t *testing.T) {
	mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

	mockBlockedEmailRepo.EXPECT().IsEmailBlocked(context.Background(), "blocked@example.com").Return(true, nil)

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "blocked@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrEmailBlocked)
}

func TestAdminCreateUserService_CreateUser_AllValidRoles(t *testing.T) {
	roles := []string{"Admin", "Contributor", "Commentator"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
			mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

			mockBlockedEmailRepo.EXPECT().IsEmailBlocked(context.Background(), "test@example.com").Return(false, nil)
			mockUserRepo.EXPECT().CheckUsernameExists(context.Background(), "testuser").Return(false, nil)
			mockUserRepo.EXPECT().CheckEmailExists(context.Background(), "test@example.com").Return(false, nil)
			mockUserRepo.EXPECT().CreateUser(
				context.Background(),
				mock.AnythingOfType("*repository.User"),
			).RunAndReturn(func(ctx context.Context, u *repository.User) error {
				u.ID = 1
				return nil
			})

			svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo)
			result, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Role:     role,
			})

			require.NoError(t, err)
			assert.Equal(t, role, result.User.Role)
		})
	}
}

func TestAdminCreateUserService_CreateUser_RepoError(t *testing.T) {
	mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

	mockBlockedEmailRepo.EXPECT().IsEmailBlocked(context.Background(), "test@example.com").Return(false, nil)
	mockUserRepo.EXPECT().CheckUsernameExists(context.Background(), "testuser").Return(false, errors.New("db error"))

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrAdminCreateFailed)
}
