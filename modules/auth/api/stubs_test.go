package api_test

import (
	"context"
	"errors"
	"time"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// stubPolicy is a test double for the CredentialPolicy seam. Validation
// failures are injected per test; hashing is deterministic
// ("hashed:"+password) so verify tests distinguish right from wrong
// passwords without real crypto.
type stubPolicy struct {
	emailErr    error
	usernameErr error
	passwordErr error
	verifyErr   error
	hashErr     error
	genErr      error
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

func (s stubPolicy) NeedsRehash(_ string) bool {
	return false
}

// stubUserStore is a test double for the api userStore seam.
type stubUserStore struct {
	byUsername     map[string]*userdomain.User
	byEmail        map[string]*userdomain.User
	byID           map[int]*userdomain.User
	admin          *userdomain.User
	getErr         error
	updateErr      error
	created        *userdomain.User
	createErr      error
	usernameExists map[string]bool
	emailExists    map[string]bool
	statusUpdated  bool
}

func (s *stubUserStore) GetUserByUsername(_ context.Context, username string) (*userdomain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.byUsername[username], nil
}

func (s *stubUserStore) GetUserByEmail(_ context.Context, email string) (*userdomain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.byEmail[email], nil
}

func (s *stubUserStore) GetUserByID(_ context.Context, userID int) (*userdomain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.admin != nil && s.admin.ID == userID {
		return s.admin, nil
	}
	return s.byID[userID], nil
}

func (s *stubUserStore) GetAdminUser(_ context.Context) (*userdomain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.admin, nil
}

func (s *stubUserStore) UpdateAdminPasswordAndEmail(_ context.Context, passwordHash, email, _ string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if s.admin != nil {
		s.admin.PasswordHash = passwordHash
		s.admin.Email = email
		s.admin.Status = "verified"
	}
	return nil
}

func (s *stubUserStore) CreateUser(_ context.Context, user *userdomain.User) error {
	if s.createErr != nil {
		return s.createErr
	}
	user.ID = 123
	s.created = user
	return nil
}

func (s *stubUserStore) CheckUsernameExists(_ context.Context, username string) (bool, error) {
	return s.usernameExists[username], nil
}

func (s *stubUserStore) CheckEmailExists(_ context.Context, email string) (bool, error) {
	return s.emailExists[email], nil
}

func (s *stubUserStore) SetEmailVerified(_ context.Context, userID int, emailVerified bool) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if u, ok := s.byID[userID]; ok && u != nil {
		u.EmailVerified = emailVerified
	}
	return nil
}

func (s *stubUserStore) UpdateUserStatus(_ context.Context, userID int, status string) error {
	s.statusUpdated = true
	if s.updateErr != nil {
		return s.updateErr
	}
	if u, ok := s.byID[userID]; ok && u != nil {
		u.Status = status
	}
	return nil
}

func (s *stubUserStore) UpdatePasswordByUserID(_ context.Context, _ int, _ string) error {
	return s.updateErr
}

func (s *stubUserStore) UpdateLastLoginAt(_ context.Context, _ int) error {
	return s.updateErr
}

// stubFailedStore is a stateful test double for the failed-login seam.
type stubFailedStore struct {
	attempts     map[int]int
	locked       map[int]bool
	getErr       error
	createErr    error
	incrementErr error
	lockErr      error
	resetErr     error
	isLockedErr  error
	wasReset     bool
	wasLocked    bool
}

func (s *stubFailedStore) GetByUserID(_ context.Context, userID int) (*authdomain.FailedLoginAttempt, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	n, ok := s.attempts[userID]
	if !ok {
		return nil, authdomain.ErrFailedLoginAttemptNotFound
	}
	return &authdomain.FailedLoginAttempt{ID: 1, UserID: userID, Attempts: n}, nil
}

func (s *stubFailedStore) Create(_ context.Context, attempt *authdomain.FailedLoginAttempt) error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.attempts == nil {
		s.attempts = map[int]int{}
	}
	s.attempts[attempt.UserID] = attempt.Attempts
	return nil
}

func (s *stubFailedStore) IncrementAttempts(_ context.Context, userID int) error {
	if s.incrementErr != nil {
		return s.incrementErr
	}
	if s.attempts == nil {
		s.attempts = map[int]int{}
	}
	s.attempts[userID]++
	return nil
}

func (s *stubFailedStore) LockAccount(_ context.Context, userID int, _ time.Duration) error {
	if s.lockErr != nil {
		return s.lockErr
	}
	if s.locked == nil {
		s.locked = map[int]bool{}
	}
	s.locked[userID] = true
	s.wasLocked = true
	return nil
}

func (s *stubFailedStore) ResetAttempts(_ context.Context, _ int) error {
	if s.resetErr != nil {
		return s.resetErr
	}
	s.wasReset = true
	return nil
}

func (s *stubFailedStore) IsLocked(_ context.Context, _ int) (bool, *time.Time, error) {
	return false, nil, s.isLockedErr
}

func (s *stubFailedStore) Delete(_ context.Context, _ int) error {
	return nil
}

func (s *stubFailedStore) UpdateLastEmailSent(_ context.Context, _ int) error {
	return nil
}

func (s *stubFailedStore) ShouldSendLockoutEmail(_ context.Context, _ int, _ time.Duration) (bool, error) {
	return false, nil
}

// stubTokenStore is a test double for the verification token seam. Tokens
// persist by hash so create-then-verify roundtrips work like production.
type stubTokenStore struct {
	tokens    map[string]*authdomain.VerificationToken
	findErr   error
	deleteErr error
	createErr error
	deleted   bool
}

func (s *stubTokenStore) CreateToken(_ context.Context, token *authdomain.VerificationToken) error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.tokens == nil {
		s.tokens = map[string]*authdomain.VerificationToken{}
	}
	s.tokens[token.TokenHash] = token
	return nil
}

func (s *stubTokenStore) FindValidToken(_ context.Context, tokenHash string) (*authdomain.VerificationToken, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.tokens[tokenHash], nil
}

func (s *stubTokenStore) DeleteUserTokens(_ context.Context, _ int) error {
	s.deleted = true
	return s.deleteErr
}

func (s *stubTokenStore) DeleteExpiredTokens(_ context.Context) error {
	return s.deleteErr
}

// stubResetStore is a test double for the password-reset token seam.
type stubResetStore struct {
	token     *authdomain.PasswordResetToken
	findErr   error
	deleteErr error
	createErr error
}

func (s *stubResetStore) CreateToken(_ context.Context, _ *authdomain.PasswordResetToken) error {
	return s.createErr
}

func (s *stubResetStore) FindValidToken(_ context.Context, _ string) (*authdomain.PasswordResetToken, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.token, nil
}

func (s *stubResetStore) DeleteUserTokens(_ context.Context, _ int) error {
	return s.deleteErr
}

// sentMail records one mailer send.
type sentMail struct {
	to       string
	username string
	token    string
}

// stubMailer is a test double for the Mailer seam. Every send is recorded
// so tests prove notifications fire.
type stubMailer struct {
	err  error
	sent []sentMail
}

func (s *stubMailer) SendVerificationEmail(_ context.Context, to, username, token string) error {
	s.sent = append(s.sent, sentMail{to: to, username: username, token: token})
	return s.err
}

func (s *stubMailer) SendPasswordResetEmail(_ context.Context, to, username, token string) error {
	s.sent = append(s.sent, sentMail{to: to, username: username, token: token})
	return s.err
}

func (s *stubMailer) SendAccountLockoutEmail(_ context.Context, to, username string) error {
	s.sent = append(s.sent, sentMail{to: to, username: username})
	return s.err
}

// stubRecorder is a test double for the NotificationRecorder seam.
type stubRecorder struct {
	registered []string
	verified   []string
	err        error
}

func (s *stubRecorder) RecordUserRegistered(_ context.Context, username, _ string) error {
	if s.err != nil {
		return s.err
	}
	s.registered = append(s.registered, username)
	return nil
}

func (s *stubRecorder) RecordUserVerified(_ context.Context, username, _ string) error {
	if s.err != nil {
		return s.err
	}
	s.verified = append(s.verified, username)
	return nil
}

// stubBlockedChecker is a test double for the BlockedEmailChecker seam.
type stubBlockedChecker struct {
	blocked map[string]bool
	err     error
	checked bool
}

func (s *stubBlockedChecker) IsEmailBlocked(_ context.Context, email string) (bool, error) {
	s.checked = true
	if s.err != nil {
		return false, s.err
	}
	return s.blocked[email], nil
}

// stubIssuer is a test double for the TokenIssuer seam.
type stubIssuer struct {
	token string
	err   error
}

func (s *stubIssuer) GenerateToken(_, _, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.token != "" {
		return s.token, nil
	}
	return "test-jwt-token", nil
}
