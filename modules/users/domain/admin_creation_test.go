package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/mocks"
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
		mock.AnythingOfType("*user.User"),
	).RunAndReturn(func(ctx context.Context, u *user.User) error {
		u.ID = 42
		return nil
	})

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, stubPolicy{}, true)
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
	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{usernameErr: errors.New("bad username")}, true)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "invalid user!",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrUsernameInvalid)
}

func TestAdminCreateUserService_CreateUser_InvalidEmail(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{emailErr: errors.New("bad email")}, true)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "not-an-email",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrEmailInvalid)
}

func TestAdminCreateUserService_CreateUser_InvalidRole(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{}, true)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "SuperAdmin",
	})

	assert.ErrorIs(t, err, user.ErrInvalidRole)
}

func TestAdminCreateUserService_CreateUser_EmptyUsername(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{usernameErr: errors.New("bad username")}, true)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrUsernameInvalid)
}

func TestAdminCreateUserService_CreateUser_EmptyEmail(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{emailErr: errors.New("bad email")}, true)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrEmailInvalid)
}

func TestAdminCreateUserService_CreateUser_EmptyRole(t *testing.T) {
	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{}, true)

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

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, stubPolicy{}, true)

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

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, stubPolicy{}, true)

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

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, stubPolicy{}, true)

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
				mock.AnythingOfType("*user.User"),
			).RunAndReturn(func(ctx context.Context, u *user.User) error {
				u.ID = 1
				return nil
			})

			svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, stubPolicy{}, true)
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

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, stubPolicy{}, true)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Contributor",
	})

	assert.ErrorIs(t, err, user.ErrAdminCreateFailed)
}

func TestAdminCreateUserService_CreateUser_CommentatorRejectedWhenCommentsDisabled(t *testing.T) {
	mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

	svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, stubPolicy{}, false)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Commentator",
	})

	// The role gate rejects before any repo call happens.
	mockUserRepo.AssertNotCalled(t, "CheckUsernameExists", context.Background(), "testuser")
	assert.ErrorIs(t, err, user.ErrInvalidRole)
}

func TestAdminCreateUserService_CreateUser_RoleServiceAllowsCustomRole(t *testing.T) {
	mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
	mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

	mockBlockedEmailRepo.EXPECT().IsEmailBlocked(context.Background(), "test@example.com").Return(false, nil)
	mockUserRepo.EXPECT().CheckUsernameExists(context.Background(), "testuser").Return(false, nil)
	mockUserRepo.EXPECT().CheckEmailExists(context.Background(), "test@example.com").Return(false, nil)
	mockUserRepo.EXPECT().CreateUser(
		context.Background(),
		mock.AnythingOfType("*user.User"),
	).RunAndReturn(func(ctx context.Context, u *user.User) error {
		u.ID = 7
		return nil
	})

	roleService := &stubRoles{assignable: map[string]bool{"Journalist": true}}

	svc := user.NewAdminCreateUserService(
		mockUserRepo,
		mockBlockedEmailRepo,
		stubPolicy{},
		true,
		user.WithRoleService(roleService),
	)

	result, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Journalist",
	})

	require.NoError(t, err)
	assert.Equal(t, "Journalist", result.User.Role)
}

func TestAdminCreateUserService_CreateUser_RoleServiceRejectsUnknown(t *testing.T) {
	roleService := &stubRoles{}

	svc := user.NewAdminCreateUserService(
		nil,
		nil,
		stubPolicy{},
		true,
		user.WithRoleService(roleService),
	)

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Ghost",
	})

	assert.ErrorIs(t, err, user.ErrInvalidRole)
}

func TestAdminCreateUserService_CreateUser_RoleServiceGatesCommentatorByComments(t *testing.T) {
	roleService := &stubRoles{}

	// Comments disabled: even with a role service, Commentator is not assignable.
	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{}, false, user.WithRoleService(roleService))

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Commentator",
	})

	assert.ErrorIs(t, err, user.ErrInvalidRole)
}

func TestAdminCreateUserService_CreateUser_RoleServiceAssignableCommentatorGated(t *testing.T) {
	// Registry allows Commentator, but the comments gate still applies: with
	// comments disabled the role service path must reject it.
	roleService := &stubRoles{assignable: map[string]bool{"Commentator": true}}

	svc := user.NewAdminCreateUserService(nil, nil, stubPolicy{}, false, user.WithRoleService(roleService))

	_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "Commentator",
	})

	assert.ErrorIs(t, err, user.ErrInvalidRole)
}

func TestAdminCreateUserService_CreateUser_DependencyFailures(t *testing.T) {
	tests := []struct {
		name             string
		blockedErr       error
		usernameCheckErr error
		emailCheckErr    error
		policy           stubPolicy
		createErr        error
		wantErr          bool
	}{
		{
			name:       "error - blocked list check fails",
			blockedErr: assert.AnError,
			wantErr:    true,
		},
		{
			name:             "error - username check fails",
			usernameCheckErr: assert.AnError,
			wantErr:          true,
		},
		{
			name:          "error - email check fails",
			emailCheckErr: assert.AnError,
			wantErr:       true,
		},
		{
			name:    "error - password generation fails",
			policy:  stubPolicy{genErr: assert.AnError},
			wantErr: true,
		},
		{
			name:    "error - password hashing fails",
			policy:  stubPolicy{hashErr: assert.AnError},
			wantErr: true,
		},
		{
			name:      "error - user creation fails",
			createErr: assert.AnError,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := mocks.NewMockAdminCreateUserRepo(t)
			mockBlockedEmailRepo := mocks.NewMockBlockedEmailRepo(t)

			mockBlockedEmailRepo.EXPECT().
				IsEmailBlocked(context.Background(), "test@example.com").
				Return(false, tt.blockedErr)
			if tt.blockedErr == nil {
				mockUserRepo.EXPECT().
					CheckUsernameExists(context.Background(), "testuser").
					Return(false, tt.usernameCheckErr)
			}
			if tt.blockedErr == nil && tt.usernameCheckErr == nil {
				mockUserRepo.EXPECT().
					CheckEmailExists(context.Background(), "test@example.com").
					Return(false, tt.emailCheckErr)
			}
			if tt.blockedErr == nil && tt.usernameCheckErr == nil && tt.emailCheckErr == nil && tt.policy.genErr == nil && tt.policy.hashErr == nil {
				mockUserRepo.EXPECT().
					CreateUser(context.Background(), mock.AnythingOfType("*user.User")).
					Return(tt.createErr)
			}

			svc := user.NewAdminCreateUserService(mockUserRepo, mockBlockedEmailRepo, tt.policy, true)

			_, err := svc.CreateUser(context.Background(), user.AdminCreateUserRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Role:     "Contributor",
			})

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, user.ErrAdminCreateFailed)
				return
			}
			require.NoError(t, err)
		})
	}
}
