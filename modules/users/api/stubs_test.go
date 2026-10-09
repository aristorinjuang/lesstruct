package api_test

import (
	"context"
	"io"
	"net/http"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	"github.com/aristorinjuang/lesstruct/modules/users/api"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// stubUserRepo implements userdomain.UserRepo, userdomain.AdminCreateUserRepo
// and api.UserStore with programmable results.
type stubUserRepo struct {
	users              map[int]*userdomain.User
	pending            []*userdomain.User
	all                []*userdomain.User
	total              int
	statuses           map[int]string
	getErr             error
	updateErr          error
	deleteErr          error
	suspendErr         error
	unsuspendErr       error
	softDeleteErr      error
	usernameExists     map[string]bool
	emailExists        map[string]bool
	emailTakenForOther map[string]bool
	created            *userdomain.User
	createErr          error
}

func (s *stubUserRepo) GetUserByID(_ context.Context, userID int) (*userdomain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if u, ok := s.users[userID]; ok {
		return u, nil
	}
	return nil, userdomain.ErrUserNotFound
}

func (s *stubUserRepo) GetPendingUsers(_ context.Context, _ int, _ int) ([]*userdomain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.pending, nil
}

func (s *stubUserRepo) GetAllUsers(_ context.Context, _ string, _ int, _ int) ([]*userdomain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.all, nil
}

func (s *stubUserRepo) CountUsers(_ context.Context, _ string) (int, error) {
	if s.getErr != nil {
		return 0, s.getErr
	}
	return s.total, nil
}

func (s *stubUserRepo) GetUserStatus(_ context.Context, userID int) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	if st, ok := s.statuses[userID]; ok {
		return st, nil
	}
	if u, ok := s.users[userID]; ok {
		return u.Status, nil
	}
	return "", userdomain.ErrUserNotFound
}

func (s *stubUserRepo) UpdateUserStatus(_ context.Context, userID int, status string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if u, ok := s.users[userID]; ok {
		u.Status = status
	}
	return nil
}

func (s *stubUserRepo) UpdateUserStatusIfCurrentStatus(_ context.Context, userID int, currentStatus, newStatus string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if u, ok := s.users[userID]; ok {
		if u.Status != currentStatus {
			return userdomain.ErrInvalidStatus
		}
		u.Status = newStatus
	}
	return nil
}

func (s *stubUserRepo) DeleteUser(_ context.Context, userID int) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.users, userID)
	return nil
}

func (s *stubUserRepo) SuspendUser(_ context.Context, userID int) error {
	if s.suspendErr != nil {
		return s.suspendErr
	}
	return s.UpdateUserStatus(context.Background(), userID, "suspended")
}

func (s *stubUserRepo) UnsuspendUser(_ context.Context, userID int) error {
	if s.unsuspendErr != nil {
		return s.unsuspendErr
	}
	return s.UpdateUserStatus(context.Background(), userID, "verified")
}

func (s *stubUserRepo) SoftDeleteUser(_ context.Context, userID int) error {
	if s.softDeleteErr != nil {
		return s.softDeleteErr
	}
	return s.UpdateUserStatus(context.Background(), userID, "soft_deleted")
}

func (s *stubUserRepo) UpdateProfile(_ context.Context, userID int, name, email, role string, customFields map[string]any) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if u, ok := s.users[userID]; ok {
		u.Name = name
		u.Email = email
		u.Role = role
		u.CustomFields = customFields
	}
	return nil
}

func (s *stubUserRepo) SetEmailVerified(_ context.Context, userID int, emailVerified bool) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if u, ok := s.users[userID]; ok {
		u.EmailVerified = emailVerified
	}
	return nil
}

func (s *stubUserRepo) CheckEmailExistsForOtherUser(_ context.Context, userID int, email string) (bool, error) {
	if s.emailTakenForOther[email] {
		return true, nil
	}
	return false, nil
}

func (s *stubUserRepo) CheckUsernameExists(_ context.Context, username string) (bool, error) {
	return s.usernameExists[username], nil
}

func (s *stubUserRepo) CheckEmailExists(_ context.Context, email string) (bool, error) {
	return s.emailExists[email], nil
}

func (s *stubUserRepo) CreateUser(_ context.Context, user *userdomain.User) error {
	if s.createErr != nil {
		return s.createErr
	}
	user.ID = 123
	s.created = user
	if s.users == nil {
		s.users = map[int]*userdomain.User{}
	}
	s.users[user.ID] = user
	return nil
}

// stubBlockedRepo implements userdomain.BlockedEmailRepo.
type stubBlockedRepo struct {
	blocked map[string]bool
}

func (s *stubBlockedRepo) BlockEmail(_ context.Context, email, _ string) error {
	if s.blocked == nil {
		s.blocked = map[string]bool{}
	}
	s.blocked[email] = true
	return nil
}

func (s *stubBlockedRepo) IsEmailBlocked(_ context.Context, email string) (bool, error) {
	return s.blocked[email], nil
}

func (s *stubBlockedRepo) UnblockEmail(_ context.Context, email string) error {
	delete(s.blocked, email)
	return nil
}

// stubPolicy implements userdomain.CredentialPolicy with injectable failures.
type stubPolicy struct {
	emailErr    error
	usernameErr error
	passwordErr error
	verifyErr   error
}

func (s stubPolicy) ValidateEmail(_ string) error    { return s.emailErr }
func (s stubPolicy) ValidateUsername(_ string) error { return s.usernameErr }
func (s stubPolicy) ValidatePassword(_ string) error { return s.passwordErr }
func (s stubPolicy) GeneratePassword(_ int) (string, error) {
	return "GeneratedPass123!", nil
}
func (s stubPolicy) HashPassword(password string) (string, error) {
	return "hashed:" + password, nil
}
func (s stubPolicy) VerifyPassword(_, _ string) error { return s.verifyErr }

// stubProfileService implements userdomain.ProfileServiceInterface.
type stubProfileService struct {
	profile     *userdomain.Profile
	profileErr  error
	updateErr   error
	passwordErr error
	export      *userdomain.UserDataExport
	exportErr   error
	verifyID    int
	verifyEmail string
	verifyErr   error
	customErr   error
	nameErr     error
}

func (s *stubProfileService) GetProfile(_ context.Context, _ kernel.Principal) (*userdomain.Profile, error) {
	return s.profile, s.profileErr
}

func (s *stubProfileService) UpdateEmail(_ context.Context, _ kernel.Principal, _, _ string) error {
	return s.updateErr
}

func (s *stubProfileService) UpdateName(_ context.Context, _ kernel.Principal, _ string) error {
	return s.nameErr
}

func (s *stubProfileService) ChangePassword(_ context.Context, _ kernel.Principal, _, _ string) error {
	return s.passwordErr
}

func (s *stubProfileService) ExportUserData(_ context.Context, _ kernel.Principal) (*userdomain.UserDataExport, error) {
	return s.export, s.exportErr
}

func (s *stubProfileService) VerifyEmailUpdate(_ context.Context, _ string) (int, string, error) {
	return s.verifyID, s.verifyEmail, s.verifyErr
}

func (s *stubProfileService) UpdateCustomFields(_ context.Context, _ kernel.Principal, _ map[string]any, _ bool) error {
	return s.customErr
}

// stubAccountDeletion implements userdomain.AccountDeletionServiceInterface.
type stubAccountDeletion struct {
	deleteErr  error
	confirmErr error
}

func (s *stubAccountDeletion) DeleteAccount(_ context.Context, _ kernel.Principal, _ string) error {
	return s.deleteErr
}

func (s *stubAccountDeletion) ValidateConfirmationString(_ string) error {
	return s.confirmErr
}

// sentMail records one mailer call.
type sentMail struct {
	to       string
	username string
	extra    string
}

// stubMailer implements api.Mailer, recording every send.
type stubMailer struct {
	sent []sentMail
	err  error
}

func (s *stubMailer) record(to, username, extra string) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, sentMail{to: to, username: username, extra: extra})
	return nil
}

func (s *stubMailer) SendUserApprovedEmail(_ context.Context, to, username string) error {
	return s.record(to, username, "")
}

func (s *stubMailer) SendUserRejectedEmail(_ context.Context, to, username string) error {
	return s.record(to, username, "")
}

func (s *stubMailer) SendUserSuspendedEmail(_ context.Context, to, username, reason string) error {
	return s.record(to, username, reason)
}

func (s *stubMailer) SendUserUnsuspendedEmail(_ context.Context, to, username string) error {
	return s.record(to, username, "")
}

func (s *stubMailer) SendUserSoftDeletedEmail(_ context.Context, to, username, reason string) error {
	return s.record(to, username, reason)
}

func (s *stubMailer) SendAccountCreatedEmail(_ context.Context, to, username, plainPassword string) error {
	return s.record(to, username, plainPassword)
}

// stubSoftDelete implements api.SoftDeleteStore.
type stubSoftDelete struct {
	byUser   map[int][]*api.SoftDeletedContent
	byID     map[int]*api.SoftDeletedContent
	byIDErr  error
	restored []string
}

func (s *stubSoftDelete) GetSoftDeletedContentByUser(_ context.Context, userID int) ([]*api.SoftDeletedContent, error) {
	return s.byUser[userID], nil
}

func (s *stubSoftDelete) GetSoftDeletedContentByID(_ context.Context, id int) (*api.SoftDeletedContent, error) {
	if s.byIDErr != nil {
		return nil, s.byIDErr
	}
	return s.byID[id], nil
}

func (s *stubSoftDelete) RestoreContent(_ context.Context, contentType string, contentID int) error {
	s.restored = append(s.restored, contentType)
	return nil
}

// stubUserFields implements the profile user-fields provider seam.
type stubUserFields struct {
	fields       []customfield.FieldSchema
	systemFields []customfield.FieldSchema
}

func (s *stubUserFields) GetUserFields() []customfield.FieldSchema {
	return s.fields
}

func (s *stubUserFields) GetUserSystemFields() []customfield.FieldSchema {
	return s.systemFields
}

// stubPictureRepo implements profilepicture.UserRepo with programmable results.
type stubPictureRepo struct {
	picture   string
	getErr    error
	updated   string
	updateErr error
	cleared   bool
	clearErr  error
}

func (s *stubPictureRepo) GetUserByID(_ context.Context, _ int) (string, error) {
	return s.picture, s.getErr
}

func (s *stubPictureRepo) UpdateProfilePicture(_ context.Context, _ int, picture string) error {
	s.updated = picture
	return s.updateErr
}

func (s *stubPictureRepo) DeleteProfilePicture(_ context.Context, _ int) error {
	s.cleared = true
	return s.clearErr
}

// stubPictureStorage is an in-memory kernel storage for picture tests.
type stubPictureStorage struct {
	files map[string][]byte
}

func (s *stubPictureStorage) Save(filename string, reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[filename] = data
	return filename, nil
}

func (s *stubPictureStorage) Delete(filePath string) error {
	delete(s.files, filePath)
	return nil
}

func (s *stubPictureStorage) GetURL(filePath string) string {
	return "/uploads/profile_pictures/" + filePath
}

// requestWithPrincipal attaches a kernel principal to the request context,
// mirroring what the auth middleware populates.
func requestWithPrincipal(r *http.Request, id, username string, roles ...string) *http.Request {
	return r.WithContext(kernel.ContextWithPrincipal(r.Context(), kernel.Principal{
		ID:       id,
		Username: username,
		Roles:    roles,
	}))
}
