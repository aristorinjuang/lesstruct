package user_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSystemFieldProvider struct {
	slugs []string
}

func (m *mockSystemFieldProvider) GetUserSystemFieldSlugs() []string {
	return m.slugs
}

func principal(userID, username string) kernel.Principal {
	return kernel.Principal{ID: userID, Username: username}
}

func newService(
	t *testing.T,
	store *stubStore,
	mailer *stubMailer,
	policy stubPolicy,
) *user.ProfileService {
	t.Helper()
	return user.NewProfileService(
		store,
		store,
		store,
		mailer,
		policy,
		nil,
	)
}

func newServiceWithSystemFields(
	t *testing.T,
	store *stubStore,
	mailer *stubMailer,
	policy stubPolicy,
	provider *mockSystemFieldProvider,
) *user.ProfileService {
	t.Helper()
	return user.NewProfileService(
		store,
		store,
		store,
		mailer,
		policy,
		provider,
	)
}

func TestProfileService_GetProfile_Success(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				Username:     "testuser",
				Name:         "Test User",
				Email:        "test@example.com",
				Role:         "Author",
				CustomFields: map[string]any{"job_title": "Engineer"},
			},
		},
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	profile, err := service.GetProfile(context.Background(), principal("123", "testuser"))
	require.NoError(t, err)

	assert.Equal(t, 123, profile.ID)
	assert.Equal(t, "testuser", profile.Username)
	assert.Equal(t, "Test User", profile.Name)
	assert.Equal(t, "test@example.com", profile.Email)
	assert.Equal(t, "Author", profile.Role)
	assert.Equal(t, map[string]any{"job_title": "Engineer"}, profile.CustomFields)
}

func TestProfileService_GetProfile_NoCustomFields(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			456: {
				ID:       456,
				Username: "nosluguser",
				Name:     "No Slug",
				Email:    "noslug@example.com",
				Role:     "Author",
			},
		},
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	profile, err := service.GetProfile(context.Background(), principal("456", "testuser"))
	require.NoError(t, err)

	assert.Nil(t, profile.CustomFields)
}

func TestProfileService_GetProfile_UserNotFound(t *testing.T) {
	store := &stubStore{
		getErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	_, err := service.GetProfile(context.Background(), principal("123", "testuser"))
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_Success(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				Username:     "testuser",
				PasswordHash: "hashed:CurrentPassword123!",
			},
		},
		emailExists: map[string]bool{},
	}
	mailer := &stubMailer{}

	service := newService(t, store, mailer, stubPolicy{})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "newemail@example.com", "CurrentPassword123!")
	require.NoError(t, err)
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "newemail@example.com", mailer.sent[0].to)
	assert.Equal(t, "testuser", mailer.sent[0].username)
	assert.NotEmpty(t, mailer.sent[0].token)
	assert.NotEmpty(t, store.createdToken)
}

func TestProfileService_UpdateEmail_InvalidEmail(t *testing.T) {
	store := &stubStore{}
	mailer := &stubMailer{}

	service := newService(t, store, mailer, stubPolicy{emailErr: assert.AnError})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "invalid-email", "CurrentPassword123!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidEmail)
	assert.Empty(t, mailer.sent)
}

func TestProfileService_UpdateEmail_EmailAlreadyInUse(t *testing.T) {
	store := &stubStore{
		emailExists: map[string]bool{"existing@example.com": true},
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "existing@example.com", "CurrentPassword123!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrEmailAlreadyInUse)
}

func TestProfileService_UpdateEmail_InvalidCurrentPassword(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				Username:     "testuser",
				PasswordHash: "hashed:CurrentPassword123!",
			},
		},
		emailExists: map[string]bool{},
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "newemail@example.com", "WrongPassword123!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidPassword)
}

func TestProfileService_ChangePassword_Success(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				PasswordHash: "hashed:CurrentPassword123!",
			},
		},
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.ChangePassword(context.Background(), principal("123", "testuser"), "CurrentPassword123!", "NewPassword456!")
	require.NoError(t, err)
	assert.Equal(t, "hashed:NewPassword456!", store.updatedPassword)
}

func TestProfileService_ChangePassword_InvalidNewPassword(t *testing.T) {
	store := &stubStore{}

	service := newService(t, store, &stubMailer{}, stubPolicy{passwordErr: assert.AnError})

	err := service.ChangePassword(context.Background(), principal("123", "testuser"), "CurrentPassword123!", "weak")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidPassword)
}

func TestProfileService_ChangePassword_InvalidCurrentPassword(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				PasswordHash: "hashed:CurrentPassword123!",
			},
		},
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.ChangePassword(context.Background(), principal("123", "testuser"), "WrongPassword123!", "NewPassword456!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidPassword)
}

func TestProfileService_ExportUserData_Success(t *testing.T) {
	store := &stubStore{
		exportData: &user.UserDataExport{
			User: &user.User{
				ID:       123,
				Username: "testuser",
			},
		},
	}
	mailer := &stubMailer{}

	service := newService(t, store, mailer, stubPolicy{})

	exportData, err := service.ExportUserData(context.Background(), principal("123", "testuser"))
	require.NoError(t, err)
	assert.Equal(t, 123, exportData.User.ID)
	assert.Equal(t, "testuser", exportData.User.Username)
	require.Len(t, mailer.sent, 1)
}

func TestProfileService_VerifyEmailUpdate_Success(t *testing.T) {
	store := &stubStore{
		token: &user.EmailUpdateToken{
			ID:       1,
			UserID:   123,
			NewEmail: "newemail@example.com",
		},
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	userID, newEmail, err := service.VerifyEmailUpdate(context.Background(), "validtoken")
	require.NoError(t, err)
	assert.Equal(t, 123, userID)
	assert.Equal(t, "newemail@example.com", newEmail)
}

func TestProfileService_VerifyEmailUpdate_InvalidToken(t *testing.T) {
	store := &stubStore{
		getTokenErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	_, _, err := service.VerifyEmailUpdate(context.Background(), "invalidtoken")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_CheckEmailExistsError(t *testing.T) {
	store := &stubStore{
		emailCheckErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_GetUserByIDError(t *testing.T) {
	store := &stubStore{
		emailExists: map[string]bool{},
		getErr:      assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_CreateTokenError(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				Username:     "testuser",
				PasswordHash: "hashed:CurrentPassword123!",
			},
		},
		emailExists:    map[string]bool{},
		createTokenErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_SendEmailError(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				Username:     "testuser",
				PasswordHash: "hashed:CurrentPassword123!",
			},
		},
		emailExists: map[string]bool{},
	}
	mailer := &stubMailer{err: assert.AnError}

	service := newService(t, store, mailer, stubPolicy{})

	err := service.UpdateEmail(context.Background(), principal("123", "testuser"), "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
	require.Len(t, mailer.sent, 1)
}

func TestProfileService_VerifyEmailUpdate_UpdateEmailError(t *testing.T) {
	store := &stubStore{
		token: &user.EmailUpdateToken{
			ID:       1,
			UserID:   123,
			NewEmail: "newemail@example.com",
		},
		updateErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	_, _, err := service.VerifyEmailUpdate(context.Background(), "validtoken")
	require.Error(t, err)
}

func TestProfileService_VerifyEmailUpdate_DeleteTokenError(t *testing.T) {
	store := &stubStore{
		token: &user.EmailUpdateToken{
			ID:       1,
			UserID:   123,
			NewEmail: "newemail@example.com",
		},
		deleteTokenErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	// Should succeed even if token deletion fails (error is logged but not returned)
	userID, newEmail, err := service.VerifyEmailUpdate(context.Background(), "validtoken")
	require.NoError(t, err)
	assert.Equal(t, 123, userID)
	assert.Equal(t, "newemail@example.com", newEmail)
}

func TestProfileService_ChangePassword_GetUserByIDError(t *testing.T) {
	store := &stubStore{
		getErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.ChangePassword(context.Background(), principal("123", "testuser"), "CurrentPassword123!", "NewPassword456!")
	require.Error(t, err)
}

func TestProfileService_ChangePassword_UpdatePasswordError(t *testing.T) {
	store := &stubStore{
		users: map[int]*user.User{
			123: {
				ID:           123,
				PasswordHash: "hashed:CurrentPassword123!",
			},
		},
		updateErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.ChangePassword(context.Background(), principal("123", "testuser"), "CurrentPassword123!", "NewPassword456!")
	require.Error(t, err)
}

func TestProfileService_ExportUserData_GetUserDataForExportError(t *testing.T) {
	store := &stubStore{
		exportErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	_, err := service.ExportUserData(context.Background(), principal("123", "testuser"))
	require.Error(t, err)
}

func TestProfileService_ExportUserData_SendEmailError(t *testing.T) {
	store := &stubStore{
		exportData: &user.UserDataExport{
			User: &user.User{
				ID:       123,
				Username: "testuser",
			},
		},
	}
	mailer := &stubMailer{err: assert.AnError}

	service := newService(t, store, mailer, stubPolicy{})

	// Should succeed even if email notification fails (error is logged but not returned)
	exportData, err := service.ExportUserData(context.Background(), principal("123", "testuser"))
	require.NoError(t, err)
	assert.Equal(t, 123, exportData.User.ID)
	require.Len(t, mailer.sent, 1)
}

func TestProfileService_UpdateCustomFields_StripsSystemFields(t *testing.T) {
	store := &stubStore{}

	provider := &mockSystemFieldProvider{slugs: []string{"internal_rating"}}
	service := newServiceWithSystemFields(t, store, &stubMailer{}, stubPolicy{}, provider)

	err := service.UpdateCustomFields(context.Background(), principal("123", "testuser"), map[string]any{
		"job_title":       "Engineer",
		"company":         "Acme",
		"internal_rating": "gold",
	}, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"job_title": "Engineer",
		"company":   "Acme",
	}, store.updatedFields)
}

func TestProfileService_UpdateCustomFields_AdminPreservesSystemFields(t *testing.T) {
	store := &stubStore{}

	provider := &mockSystemFieldProvider{slugs: []string{"internal_rating"}}
	service := newServiceWithSystemFields(t, store, &stubMailer{}, stubPolicy{}, provider)

	err := service.UpdateCustomFields(context.Background(), principal("123", "testuser"), map[string]any{
		"job_title":       "Engineer",
		"company":         "Acme",
		"internal_rating": "gold",
	}, true)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"job_title":       "Engineer",
		"company":         "Acme",
		"internal_rating": "gold",
	}, store.updatedFields)
}

func TestProfileService_UpdateCustomFields_NoSystemFieldsProvider(t *testing.T) {
	store := &stubStore{}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateCustomFields(context.Background(), principal("123", "testuser"), map[string]any{
		"job_title": "Engineer",
		"company":   "Acme",
	}, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"job_title": "Engineer",
		"company":   "Acme",
	}, store.updatedFields)
}

func TestProfileService_UpdateCustomFields_RepoError(t *testing.T) {
	store := &stubStore{
		updateErr: assert.AnError,
	}

	provider := &mockSystemFieldProvider{slugs: []string{"internal_rating"}}
	service := newServiceWithSystemFields(t, store, &stubMailer{}, stubPolicy{}, provider)

	err := service.UpdateCustomFields(context.Background(), principal("123", "testuser"), map[string]any{
		"job_title": "Engineer",
	}, false)
	require.Error(t, err)
}

func TestProfileService_UpdateName_Success(t *testing.T) {
	store := &stubStore{}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateName(context.Background(), principal("123", "testuser"), "Jane Doe")
	require.NoError(t, err)
	assert.Equal(t, "Jane Doe", store.updatedName)
}

func TestProfileService_UpdateName_RepoError(t *testing.T) {
	store := &stubStore{
		updateErr: assert.AnError,
	}

	service := newService(t, store, &stubMailer{}, stubPolicy{})

	err := service.UpdateName(context.Background(), principal("123", "testuser"), "Jane Doe")
	require.Error(t, err)
}

func TestProfileService_InvalidPrincipal(t *testing.T) {
	service := user.NewProfileService(nil, nil, nil, nil, stubPolicy{}, nil)
	bad := kernel.Principal{ID: "not-a-number", Username: "ghost"}

	tests := []struct {
		name    string
		call    func() error
		wantErr bool
	}{
		{
			name: "get profile rejects non-decimal id",
			call: func() error {
				_, err := service.GetProfile(context.Background(), bad)
				return err
			},
			wantErr: true,
		},
		{
			name: "update email rejects non-decimal id",
			call: func() error {
				return service.UpdateEmail(context.Background(), bad, "new@example.com", "secret")
			},
			wantErr: true,
		},
		{
			name: "update name rejects non-decimal id",
			call: func() error {
				return service.UpdateName(context.Background(), bad, "Ghost")
			},
			wantErr: true,
		},
		{
			name: "change password rejects non-decimal id",
			call: func() error {
				return service.ChangePassword(context.Background(), bad, "old", "new")
			},
			wantErr: true,
		},
		{
			name: "export user data rejects non-decimal id",
			call: func() error {
				_, err := service.ExportUserData(context.Background(), bad)
				return err
			},
			wantErr: true,
		},
		{
			name: "update custom fields rejects non-decimal id",
			call: func() error {
				return service.UpdateCustomFields(context.Background(), bad, nil, false)
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
