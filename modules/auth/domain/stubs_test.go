package auth_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
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
	needsRehash bool
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
	return s.needsRehash
}

// stubLogger records printf-style messages so tests can assert that a soft
// failure was logged without failing the request.
type stubLogger struct {
	errors []string
	infos  []string
}

func (l *stubLogger) Info(format string, args ...any) {
	l.infos = append(l.infos, fmt.Sprintf(format, args...))
}

func (l *stubLogger) Error(format string, args ...any) {
	l.errors = append(l.errors, fmt.Sprintf(format, args...))
}

// failAfterReader serves data once, then fails every Read. It lets a test
// succeed the first entropy draw (the challenge token) and fail the next
// (the emailed code).
type failAfterReader struct {
	data []byte
	read int
	err  error
}

func (r *failAfterReader) Read(p []byte) (int, error) {
	if r.read >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.read:])
	r.read += n
	return n, nil
}

// stubUserStore is a test double for the auth userStore seam. Each test
// constructs its own with exactly the data and failures its path needs.
type stubUserStore struct {
	byUsername      map[string]*userdomain.User
	byEmail         map[string]*userdomain.User
	getErr          error
	usernameExists  map[string]bool
	usernameErr     error
	emailExists     map[string]bool
	emailErr        error
	created         *userdomain.User
	createErr       error
	usernameChecked bool
	emailChecked    bool
	updateErr       error
	statusErr       error
	statusUpdated   bool
	lastLoginErr    error
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

func (s *stubUserStore) CreateUser(_ context.Context, user *userdomain.User) error {
	if s.createErr != nil {
		return s.createErr
	}
	user.ID = 123
	s.created = user
	return nil
}

func (s *stubUserStore) CheckUsernameExists(_ context.Context, username string) (bool, error) {
	s.usernameChecked = true
	if s.usernameErr != nil {
		return false, s.usernameErr
	}
	return s.usernameExists[username], nil
}

func (s *stubUserStore) CheckEmailExists(_ context.Context, email string) (bool, error) {
	s.emailChecked = true
	if s.emailErr != nil {
		return false, s.emailErr
	}
	return s.emailExists[email], nil
}

func (s *stubUserStore) SetEmailVerified(_ context.Context, _ int, _ bool) error {
	return s.updateErr
}

func (s *stubUserStore) UpdateUserStatus(_ context.Context, _ int, _ string) error {
	s.statusUpdated = true
	if s.statusErr != nil {
		return s.statusErr
	}
	return s.updateErr
}

func (s *stubUserStore) UpdatePasswordByUserID(_ context.Context, _ int, _ string) error {
	return s.updateErr
}

func (s *stubUserStore) UpdateLastLoginAt(_ context.Context, _ int) error {
	return s.lastLoginErr
}

// stubTokenStore is a test double for the verification token seam.
type stubTokenStore struct {
	token     *authdomain.VerificationToken
	findErr   error
	deleteErr error
	createErr error
	deleted   bool
}

func (s *stubTokenStore) CreateToken(_ context.Context, _ *authdomain.VerificationToken) error {
	return s.createErr
}

func (s *stubTokenStore) FindValidToken(_ context.Context, _ string) (*authdomain.VerificationToken, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.token, nil
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

// stubFailedStore is a stateful test double for the failed-login seam. It
// tracks attempts per user so multi-attempt lockout sequences run without
// mock choreography.
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
	created      bool
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
	s.created = true
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

func (s *stubFailedStore) IsLocked(_ context.Context, userID int) (bool, *time.Time, error) {
	if s.isLockedErr != nil {
		return false, nil, s.isLockedErr
	}
	if !s.locked[userID] {
		return false, nil, nil
	}
	until := time.Now().Add(15 * time.Minute)
	return true, &until, nil
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

// stubMFAStore is a test double for the login MFA challenge seam. It returns
// one configurable challenge and records the mutations the service performs.
type stubMFAStore struct {
	challenge   *authdomain.MFAChallenge
	created     *authdomain.MFAChallenge
	createdAtID int
	findErr     error
	createErr   error
	updateErr   error
	deleteErr   error
	consumeMiss bool
	incremented int
	deleted     int
	userCleared int
}

func (s *stubMFAStore) Create(_ context.Context, challenge *authdomain.MFAChallenge) error {
	if s.createErr != nil {
		return s.createErr
	}
	challenge.ID = 1
	s.created = challenge
	s.createdAtID = challenge.ID
	return nil
}

func (s *stubMFAStore) FindByTokenHash(_ context.Context, _ string) (*authdomain.MFAChallenge, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.challenge, nil
}

func (s *stubMFAStore) IncrementAttempts(_ context.Context, _ int, maxAttempts int) (bool, error) {
	if s.updateErr != nil {
		return false, s.updateErr
	}
	if s.challenge == nil || s.challenge.Attempts >= maxAttempts {
		return false, nil
	}
	s.incremented++
	s.challenge.Attempts++
	return true, nil
}

func (s *stubMFAStore) Consume(_ context.Context, _ int) (bool, error) {
	if s.deleteErr != nil {
		return false, s.deleteErr
	}
	if s.consumeMiss {
		return false, nil
	}
	s.deleted++
	s.challenge = nil
	return true, nil
}

func (s *stubMFAStore) Delete(_ context.Context, _ int) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted++
	s.challenge = nil
	return nil
}

func (s *stubMFAStore) DeleteUserChallenges(_ context.Context, userID int) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.userCleared = userID
	return nil
}

func (s *stubMFAStore) DeleteExpired(_ context.Context) error {
	return s.deleteErr
}
