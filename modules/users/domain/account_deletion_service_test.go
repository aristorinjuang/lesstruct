package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAccountDeletionService(
	t *testing.T,
	store *stubStore,
	mailer *stubMailer,
) *user.AccountDeletionService {
	t.Helper()
	return user.NewAccountDeletionService(
		store,
		store,
		mailer,
		nil,
	)
}

func TestAccountDeletionService_DeleteAccount_Success(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			1: {ID: 1, Role: "User"},
		},
	}
	mailer := &stubMailer{}

	service := newAccountDeletionService(t, store, mailer)

	err := service.DeleteAccount(context.Background(), principal("1", "testuser"), "test@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "test@example.com", mailer.sent[0].to)
	assert.Equal(t, "testuser", mailer.sent[0].username)
}

func TestAccountDeletionService_DeleteAccount_LastAdmin(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			1: {ID: 1, Role: "Admin"},
		},
		adminCount: 1,
	}

	service := newAccountDeletionService(t, store, &stubMailer{})

	err := service.DeleteAccount(context.Background(), principal("1", "admin"), "admin@example.com")
	require.ErrorIs(t, err, user.ErrLastAdminDeletionForbidden)
}

func TestAccountDeletionService_DeleteAccount_AdminWithMultipleAdmins(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			1: {ID: 1, Role: "Admin"},
		},
		adminCount: 2,
	}
	mailer := &stubMailer{}

	service := newAccountDeletionService(t, store, mailer)

	err := service.DeleteAccount(context.Background(), principal("1", "admin"), "admin@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "admin@example.com", mailer.sent[0].to)
	assert.Equal(t, "admin", mailer.sent[0].username)
}

func TestAccountDeletionService_DeleteAccount_NonAdminUser(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			1: {ID: 1, Role: "User"},
		},
	}
	mailer := &stubMailer{}

	service := newAccountDeletionService(t, store, mailer)

	err := service.DeleteAccount(context.Background(), principal("1", "testuser"), "test@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.sent, 1)
}

func TestAccountDeletionService_DeleteAccount_UserNotFound(t *testing.T) {
	store := &stubStore{
		getErr: errors.New("user not found"),
	}

	service := newAccountDeletionService(t, store, &stubMailer{})

	err := service.DeleteAccount(context.Background(), principal("999", "unknown"), "unknown@example.com")
	require.Error(t, err)
}

func TestAccountDeletionService_DeleteAccount_DeleteAllUserDataError(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			1: {ID: 1, Role: "User"},
		},
		deleteErr: errors.New("database error"),
	}

	service := newAccountDeletionService(t, store, &stubMailer{})

	err := service.DeleteAccount(context.Background(), principal("1", "testuser"), "test@example.com")
	require.Error(t, err)
}

func TestAccountDeletionService_DeleteAccount_EmailError(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			1: {ID: 1, Role: "User"},
		},
	}
	mailer := &stubMailer{err: errors.New("email service error")}

	service := newAccountDeletionService(t, store, mailer)

	// Email error should not fail the deletion (log but continue)
	err := service.DeleteAccount(context.Background(), principal("1", "testuser"), "test@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.sent, 1)
}

func TestAccountDeletionService_DeleteAccount_CountAdminsError(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			1: {ID: 1, Role: "Admin"},
		},
		adminCountErr: errors.New("database error counting admins"),
	}

	service := newAccountDeletionService(t, store, &stubMailer{})

	err := service.DeleteAccount(context.Background(), principal("1", "admin"), "admin@example.com")
	require.Error(t, err)
}

func TestAccountDeletionService_ValidateConfirmationString(t *testing.T) {
	service := user.NewAccountDeletionService(nil, nil, nil, nil)

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
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAccountDeletionService_InvalidPrincipal(t *testing.T) {
	service := user.NewAccountDeletionService(nil, nil, nil, nil)
	bad := kernel.Principal{ID: "not-a-number", Username: "ghost"}

	tests := []struct {
		name    string
		call    func() error
		wantErr bool
	}{
		{
			name: "delete account rejects non-decimal id",
			call: func() error {
				return service.DeleteAccount(context.Background(), bad, "ghost@example.com")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, kernel.ErrInvalidPrincipalID)
				return
			}
			require.NoError(t, err)
		})
	}
}
