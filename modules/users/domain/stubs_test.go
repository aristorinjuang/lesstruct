package user_test

import (
	"context"
	"errors"

	user "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// stubPolicy is a test double for the CredentialPolicy seam. Validation
// failures are injected per test (proving the service maps them); hashing
// is deterministic ("hashed:"+password) so verify tests distinguish right
// from wrong passwords without real crypto.
type stubPolicy struct {
	emailErr    error
	usernameErr error
	passwordErr error
	verifyErr   error
	hashErr     error
	genErr      error
	genPassword string
}

func (s stubPolicy) ValidateEmail(_ string) error {
	return s.emailErr
}

func (s stubPolicy) ValidateUsername(_ string) error {
	return s.usernameErr
}

func (s stubPolicy) ValidatePassword(_ string) error {
	return s.passwordErr
}

func (s stubPolicy) GeneratePassword(_ int) (string, error) {
	if s.genErr != nil {
		return "", s.genErr
	}
	if s.genPassword != "" {
		return s.genPassword, nil
	}
	return "0123456789abcdef", nil
}

func (s stubPolicy) HashPassword(password string) (string, error) {
	if s.hashErr != nil {
		return "", s.hashErr
	}
	return "hashed:" + password, nil
}

func (s stubPolicy) VerifyPassword(hash, password string) error {
	if s.verifyErr != nil {
		return s.verifyErr
	}
	if hash != "hashed:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

// sentMail records one mailer send.
type sentMail struct {
	to       string
	username string
	token    string
}

// stubMailer is a test double for the Mailer seam. Every send is recorded
// (even failing ones) so tests prove notifications fire; err fails sends.
type stubMailer struct {
	err  error
	sent []sentMail
}

func (s *stubMailer) SendEmailUpdateVerificationEmail(_ context.Context, to, username, token string) error {
	s.sent = append(s.sent, sentMail{to: to, username: username, token: token})
	return s.err
}

func (s *stubMailer) SendDataExportNotificationEmail(_ context.Context, to, username string) error {
	s.sent = append(s.sent, sentMail{to: to, username: username})
	return s.err
}

func (s *stubMailer) SendAccountDeletedNotification(_ context.Context, to, username string) error {
	s.sent = append(s.sent, sentMail{to: to, username: username})
	return s.err
}

// stubRoles is a test double for the RoleAssigner seam.
type stubRoles struct {
	assignable map[string]bool
}

func (s *stubRoles) IsAssignable(role string) bool {
	return s.assignable[role]
}

// stubStore is a test double for the profile and deletion store seams. Each
// test constructs its own with exactly the data and failures its path needs.
type stubStore struct {
	users           map[int]*user.User
	getErr          error
	emailExists     map[string]bool
	emailCheckErr   error
	updateErr       error
	updatedName     string
	updatedEmail    string
	updatedPassword string
	updatedFields   map[string]any
	token           *user.EmailUpdateToken
	getTokenErr     error
	createTokenErr  error
	createdToken    string
	deleteTokenErr  error
	exportData      *user.UserDataExport
	exportErr       error
	deleteErr       error
	adminCount      int
	adminCountErr   error
}

func (s *stubStore) GetUserByID(_ context.Context, userID int) (*user.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.users[userID], nil
}

func (s *stubStore) CheckEmailExists(_ context.Context, email string) (bool, error) {
	if s.emailCheckErr != nil {
		return false, s.emailCheckErr
	}
	return s.emailExists[email], nil
}

func (s *stubStore) UpdateName(_ context.Context, userID int, name string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updatedName = name
	if u, ok := s.users[userID]; ok && u != nil {
		u.Name = name
	}
	return nil
}

func (s *stubStore) UpdateEmail(_ context.Context, userID int, email string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updatedEmail = email
	if u, ok := s.users[userID]; ok && u != nil {
		u.Email = email
	}
	return nil
}

func (s *stubStore) UpdatePassword(_ context.Context, _ int, _, newPasswordHash string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updatedPassword = newPasswordHash
	return nil
}

func (s *stubStore) UpdateCustomFields(_ context.Context, _ int, customFields map[string]any) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updatedFields = customFields
	return nil
}

func (s *stubStore) CreateToken(_ context.Context, tokenHash string, _ int, _ string) error {
	if s.createTokenErr != nil {
		return s.createTokenErr
	}
	s.createdToken = tokenHash
	return nil
}

func (s *stubStore) GetToken(_ context.Context, _ string) (*user.EmailUpdateToken, error) {
	if s.getTokenErr != nil {
		return nil, s.getTokenErr
	}
	return s.token, nil
}

func (s *stubStore) DeleteToken(_ context.Context, _ int) error {
	return s.deleteTokenErr
}

func (s *stubStore) GetUserDataForExport(_ context.Context, _ int) (*user.UserDataExport, error) {
	if s.exportErr != nil {
		return nil, s.exportErr
	}
	return s.exportData, nil
}

func (s *stubStore) DeleteAllUserData(_ context.Context, _ int) error {
	return s.deleteErr
}

func (s *stubStore) CountUsersByRoleAndStatus(_ context.Context, _, _ string) (int, error) {
	if s.adminCountErr != nil {
		return 0, s.adminCountErr
	}
	return s.adminCount, nil
}
