package user_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/domain/user"
	emailmocks "github.com/aristorinjuang/lesstruct/internal/email/mocks"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	repomocks "github.com/aristorinjuang/lesstruct/internal/repository/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockSystemFieldProvider struct {
	slugs []string
}

func (m *mockSystemFieldProvider) GetUserSystemFieldSlugs() []string {
	return m.slugs
}

func newServiceWithSystemFields(
	t *testing.T,
	userRepo *repomocks.MockUserRepo,
	emailUpdateTokenRepo *repomocks.MockEmailUpdateTokenRepo,
	userDataExportRepo *repomocks.MockUserDataExportRepo,
	emailService *emailmocks.MockEmailService,
	provider *mockSystemFieldProvider,
) *user.ProfileService {
	t.Helper()
	return user.NewProfileService(
		userRepo,
		emailUpdateTokenRepo,
		userDataExportRepo,
		emailService,
		provider,
	)
}

func newService(
	t *testing.T,
	userRepo *repomocks.MockUserRepo,
	emailUpdateTokenRepo *repomocks.MockEmailUpdateTokenRepo,
	userDataExportRepo *repomocks.MockUserDataExportRepo,
	emailService *emailmocks.MockEmailService,
) *user.ProfileService {
	t.Helper()
	return user.NewProfileService(
		userRepo,
		emailUpdateTokenRepo,
		userDataExportRepo,
		emailService,
		nil,
	)
}

func TestProfileService_GetProfile_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			Name:         "Test User",
			Email:        "test@example.com",
			Role:         "Author",
			CustomFields: map[string]any{"job_title": "Engineer"},
		}, nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	profile, err := service.GetProfile(context.Background(), 123)
	require.NoError(t, err)

	assert.Equal(t, 123, profile.ID)
	assert.Equal(t, "testuser", profile.Username)
	assert.Equal(t, "Test User", profile.Name)
	assert.Equal(t, "test@example.com", profile.Email)
	assert.Equal(t, "Author", profile.Role)
	assert.Equal(t, map[string]any{"job_title": "Engineer"}, profile.CustomFields)
}

func TestProfileService_GetProfile_NoCustomFields(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 456).
		Return(&repository.User{
			ID:       456,
			Username: "nosluguser",
			Name:     "No Slug",
			Email:    "noslug@example.com",
			Role:     "Author",
		}, nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	profile, err := service.GetProfile(context.Background(), 456)
	require.NoError(t, err)

	assert.Nil(t, profile.CustomFields)
}

func TestProfileService_GetProfile_UserNotFound(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(nil, assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	_, err := service.GetProfile(context.Background(), 123)
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_Success(t *testing.T) {
	validPasswordHash, _ := auth.HashPassword("CurrentPassword123!")

	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		CheckEmailExists(context.Background(), "newemail@example.com").
		Return(false, nil)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: validPasswordHash,
		}, nil)

	emailUpdateTokenRepo.EXPECT().
		CreateToken(context.Background(), mock.Anything, 123, "newemail@example.com").
		Return(nil)

	emailService.EXPECT().
		SendEmailUpdateVerificationEmail(
			context.Background(),
			"newemail@example.com",
			"testuser",
			mock.Anything,
		).
		Return(nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "newemail@example.com", "CurrentPassword123!")
	require.NoError(t, err)
}

func TestProfileService_UpdateEmail_InvalidEmail(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "invalid-email", "CurrentPassword123!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidEmail)
}

func TestProfileService_UpdateEmail_EmailAlreadyInUse(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		CheckEmailExists(context.Background(), "existing@example.com").
		Return(true, nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "existing@example.com", "CurrentPassword123!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrEmailAlreadyInUse)
}

func TestProfileService_UpdateEmail_InvalidCurrentPassword(t *testing.T) {
	validPasswordHash, _ := auth.HashPassword("CurrentPassword123!")

	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		CheckEmailExists(context.Background(), "newemail@example.com").
		Return(false, nil)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: validPasswordHash,
		}, nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "newemail@example.com", "WrongPassword123!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidPassword)
}

func TestProfileService_ChangePassword_Success(t *testing.T) {
	validPasswordHash, _ := auth.HashPassword("CurrentPassword123!")

	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			PasswordHash: validPasswordHash,
		}, nil)

	userRepo.EXPECT().
		UpdatePassword(
			context.Background(),
			123,
			validPasswordHash,
			mock.Anything,
		).
		Return(nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.ChangePassword(context.Background(), 123, "CurrentPassword123!", "NewPassword456!")
	require.NoError(t, err)
}

func TestProfileService_ChangePassword_InvalidNewPassword(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.ChangePassword(context.Background(), 123, "CurrentPassword123!", "weak")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidPassword)
}

func TestProfileService_ChangePassword_InvalidCurrentPassword(t *testing.T) {
	validPasswordHash, _ := auth.HashPassword("CurrentPassword123!")

	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			PasswordHash: validPasswordHash,
		}, nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.ChangePassword(context.Background(), 123, "WrongPassword123!", "NewPassword456!")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrInvalidPassword)
}

func TestProfileService_ExportUserData_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userDataExportRepo.EXPECT().
		GetUserDataForExport(context.Background(), 123).
		Return(&repository.UserDataExport{
			User: &repository.User{
				ID:       123,
				Username: "testuser",
			},
		}, nil)

	emailService.EXPECT().
		SendDataExportNotificationEmail(context.Background(), "", "testuser").
		Return(nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	exportData, err := service.ExportUserData(context.Background(), 123)
	require.NoError(t, err)
	assert.Equal(t, 123, exportData.User.ID)
	assert.Equal(t, "testuser", exportData.User.Username)
}

func TestProfileService_VerifyEmailUpdate_Success(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	emailUpdateTokenRepo.EXPECT().
		GetToken(context.Background(), mock.Anything).
		Return(&repository.EmailUpdateToken{
			ID:      1,
			UserID:  123,
			NewEmail: "newemail@example.com",
		}, nil)

	userRepo.EXPECT().
		UpdateEmail(context.Background(), 123, "newemail@example.com").
		Return(nil)

	emailUpdateTokenRepo.EXPECT().
		DeleteToken(context.Background(), 1).
		Return(nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	userID, newEmail, err := service.VerifyEmailUpdate(context.Background(), "validtoken")
	require.NoError(t, err)
	assert.Equal(t, 123, userID)
	assert.Equal(t, "newemail@example.com", newEmail)
}

func TestProfileService_VerifyEmailUpdate_InvalidToken(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	emailUpdateTokenRepo.EXPECT().
		GetToken(context.Background(), mock.Anything).
		Return(nil, assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	_, _, err := service.VerifyEmailUpdate(context.Background(), "invalidtoken")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_CheckEmailExistsError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		CheckEmailExists(context.Background(), "newemail@example.com").
		Return(false, assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_GetUserByIDError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		CheckEmailExists(context.Background(), "newemail@example.com").
		Return(false, nil)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(nil, assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_CreateTokenError(t *testing.T) {
	validPasswordHash, _ := auth.HashPassword("CurrentPassword123!")

	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		CheckEmailExists(context.Background(), "newemail@example.com").
		Return(false, nil)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: validPasswordHash,
		}, nil)

	emailUpdateTokenRepo.EXPECT().
		CreateToken(context.Background(), mock.Anything, 123, "newemail@example.com").
		Return(assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
}

func TestProfileService_UpdateEmail_SendEmailError(t *testing.T) {
	validPasswordHash, _ := auth.HashPassword("CurrentPassword123!")

	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		CheckEmailExists(context.Background(), "newemail@example.com").
		Return(false, nil)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			Username:     "testuser",
			PasswordHash: validPasswordHash,
		}, nil)

	emailUpdateTokenRepo.EXPECT().
		CreateToken(context.Background(), mock.Anything, 123, "newemail@example.com").
		Return(nil)

	emailService.EXPECT().
		SendEmailUpdateVerificationEmail(
			context.Background(),
			"newemail@example.com",
			"testuser",
			mock.Anything,
		).
		Return(assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateEmail(context.Background(), 123, "newemail@example.com", "CurrentPassword123!")
	require.Error(t, err)
}

func TestProfileService_VerifyEmailUpdate_UpdateEmailError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	emailUpdateTokenRepo.EXPECT().
		GetToken(context.Background(), mock.Anything).
		Return(&repository.EmailUpdateToken{
			ID:      1,
			UserID:  123,
			NewEmail: "newemail@example.com",
		}, nil)

	userRepo.EXPECT().
		UpdateEmail(context.Background(), 123, "newemail@example.com").
		Return(assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	_, _, err := service.VerifyEmailUpdate(context.Background(), "validtoken")
	require.Error(t, err)
}

func TestProfileService_VerifyEmailUpdate_DeleteTokenError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	emailUpdateTokenRepo.EXPECT().
		GetToken(context.Background(), mock.Anything).
		Return(&repository.EmailUpdateToken{
			ID:      1,
			UserID:  123,
			NewEmail: "newemail@example.com",
		}, nil)

	userRepo.EXPECT().
		UpdateEmail(context.Background(), 123, "newemail@example.com").
		Return(nil)

	emailUpdateTokenRepo.EXPECT().
		DeleteToken(context.Background(), 1).
		Return(assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	// Should succeed even if token deletion fails (error is logged but not returned)
	userID, newEmail, err := service.VerifyEmailUpdate(context.Background(), "validtoken")
	require.NoError(t, err)
	assert.Equal(t, 123, userID)
	assert.Equal(t, "newemail@example.com", newEmail)
}

func TestProfileService_ChangePassword_GetUserByIDError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(nil, assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.ChangePassword(context.Background(), 123, "CurrentPassword123!", "NewPassword456!")
	require.Error(t, err)
}

func TestProfileService_ChangePassword_UpdatePasswordError(t *testing.T) {
	validPasswordHash, _ := auth.HashPassword("CurrentPassword123!")

	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		GetUserByID(context.Background(), 123).
		Return(&repository.User{
			ID:           123,
			PasswordHash: validPasswordHash,
		}, nil)

	userRepo.EXPECT().
		UpdatePassword(
			context.Background(),
			123,
			validPasswordHash,
			mock.Anything,
		).
		Return(assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.ChangePassword(context.Background(), 123, "CurrentPassword123!", "NewPassword456!")
	require.Error(t, err)
}

func TestProfileService_ExportUserData_GetUserDataForExportError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userDataExportRepo.EXPECT().
		GetUserDataForExport(context.Background(), 123).
		Return(nil, assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	_, err := service.ExportUserData(context.Background(), 123)
	require.Error(t, err)
}

func TestProfileService_ExportUserData_SendEmailError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userDataExportRepo.EXPECT().
		GetUserDataForExport(context.Background(), 123).
		Return(&repository.UserDataExport{
			User: &repository.User{
				ID:       123,
				Username: "testuser",
			},
		}, nil)

	emailService.EXPECT().
		SendDataExportNotificationEmail(context.Background(), "", "testuser").
		Return(assert.AnError)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	// Should succeed even if email notification fails (error is logged but not returned)
	exportData, err := service.ExportUserData(context.Background(), 123)
	require.NoError(t, err)
	assert.Equal(t, 123, exportData.User.ID)
}

func TestProfileService_UpdateCustomFields_StripsSystemFields(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		UpdateCustomFields(context.Background(), 123, map[string]any{
			"job_title": "Engineer",
			"company":   "Acme",
		}).
		Return(nil)

	provider := &mockSystemFieldProvider{slugs: []string{"internal_rating"}}
	service := newServiceWithSystemFields(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, provider)

	err := service.UpdateCustomFields(context.Background(), 123, map[string]any{
		"job_title":       "Engineer",
		"company":         "Acme",
		"internal_rating": "gold",
	}, false)
	require.NoError(t, err)
}

func TestProfileService_UpdateCustomFields_AdminPreservesSystemFields(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		UpdateCustomFields(context.Background(), 123, map[string]any{
			"job_title":       "Engineer",
			"company":         "Acme",
			"internal_rating": "gold",
		}).
		Return(nil)

	provider := &mockSystemFieldProvider{slugs: []string{"internal_rating"}}
	service := newServiceWithSystemFields(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, provider)

	err := service.UpdateCustomFields(context.Background(), 123, map[string]any{
		"job_title":       "Engineer",
		"company":         "Acme",
		"internal_rating": "gold",
	}, true)
	require.NoError(t, err)
}

func TestProfileService_UpdateCustomFields_NoSystemFieldsProvider(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		UpdateCustomFields(context.Background(), 123, map[string]any{
			"job_title": "Engineer",
			"company":   "Acme",
		}).
		Return(nil)

	service := newService(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService)

	err := service.UpdateCustomFields(context.Background(), 123, map[string]any{
		"job_title": "Engineer",
		"company":   "Acme",
	}, false)
	require.NoError(t, err)
}

func TestProfileService_UpdateCustomFields_RepoError(t *testing.T) {
	userRepo := repomocks.NewMockUserRepo(t)
	emailUpdateTokenRepo := repomocks.NewMockEmailUpdateTokenRepo(t)
	userDataExportRepo := repomocks.NewMockUserDataExportRepo(t)
	emailService := emailmocks.NewMockEmailService(t)

	userRepo.EXPECT().
		UpdateCustomFields(context.Background(), 123, map[string]any{
			"job_title": "Engineer",
		}).
		Return(assert.AnError)

	provider := &mockSystemFieldProvider{slugs: []string{"internal_rating"}}
	service := newServiceWithSystemFields(t, userRepo, emailUpdateTokenRepo, userDataExportRepo, emailService, provider)

	err := service.UpdateCustomFields(context.Background(), 123, map[string]any{
		"job_title": "Engineer",
	}, false)
	require.Error(t, err)
}

