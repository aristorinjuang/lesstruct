package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

var (
	ErrAccountLocked       = errors.New("account locked due to too many failed login attempts")
	ErrEmailNotVerified    = errors.New("email address not verified")
	ErrAccountSuspended    = errors.New("account has been suspended")
	ErrAccountDeleted      = errors.New("account has been deleted")
	ErrInvalidCredentials  = errors.New("invalid username or password")
	MaxFailedLoginAttempts = 3
	AccountLockoutDuration = 15 * time.Minute
)

// IsEmailIdentifier reports whether a login identifier is an email address
// rather than a username. Usernames are validated as [a-zA-Z0-9_-]{1,50}, so
// an "@" unambiguously marks an email identifier.
func IsEmailIdentifier(identifier string) bool {
	return strings.Contains(identifier, "@")
}

// LoginRequest represents a login request
type LoginRequest struct {
	// Username is the login identifier: a username or an email address.
	Username string
	Password string
}

// LoginResult represents the result of a successful login
type LoginResult struct {
	ID       int
	UserID   string
	Username string
	Email    string
	Role     string
}

// LoginService handles login business logic with account lockout
type LoginService struct {
	userRepo        userStore
	failedLoginRepo FailedLoginAttemptRepo
	policy          CredentialPolicy
	logger          kernel.Logger
}

// findUser resolves a login identifier to a user. Identifiers containing an
// "@" are email addresses; usernames are validated without "@", so the
// distinction is unambiguous.
func (s *LoginService) findUser(ctx context.Context, identifier string) (*userdomain.User, error) {
	if IsEmailIdentifier(identifier) {
		return s.userRepo.GetUserByEmail(ctx, identifier)
	}
	return s.userRepo.GetUserByUsername(ctx, identifier)
}

// handleFailedLogin handles a failed login attempt and implements account lockout
func (s *LoginService) handleFailedLogin(ctx context.Context, userID int) error {
	// Get current failed attempt record
	attempt, err := s.failedLoginRepo.GetByUserID(ctx, userID)
	if err != nil && err != ErrFailedLoginAttemptNotFound {
		return fmt.Errorf("failed to get attempt record: %w", err)
	}

	// If no previous attempts, this is the first one
	if err == ErrFailedLoginAttemptNotFound {
		return s.failedLoginRepo.Create(ctx, &FailedLoginAttempt{
			UserID:        userID,
			Attempts:      1,
			LastAttemptAt: time.Now(),
			LockedUntil:   nil,
		})
	}

	// Increment attempt counter
	newAttemptCount := attempt.Attempts + 1

	// Check if this is the third failed attempt - lock the account
	if newAttemptCount >= MaxFailedLoginAttempts {
		// Lock the account for 15 minutes
		err = s.failedLoginRepo.LockAccount(ctx, userID, AccountLockoutDuration)
		if err != nil {
			return fmt.Errorf("failed to lock account: %w", err)
		}
		return nil
	}

	// Increment the attempt counter
	err = s.failedLoginRepo.IncrementAttempts(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to increment attempts: %w", err)
	}

	return nil
}

// Login performs login with account lockout mechanism
func (s *LoginService) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
	// Validate input
	if req.Username == "" || req.Password == "" {
		return nil, ErrInvalidCredentials
	}

	// Get user by username or email
	user, err := s.findUser(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// User not found - generic error to prevent username enumeration
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	// Check if account is locked before attempting authentication
	isLocked, _, err := s.failedLoginRepo.IsLocked(ctx, user.ID)
	if err != nil {
		// Fail open on a lockout-store error: refusing the attempt would turn a
		// persistence problem into a 500 that is indistinguishable from a
		// username-enumeration oracle (unknown users never reach this call).
		if s.logger != nil {
			s.logger.Error("failed to check lock status: %v", err)
		}
		isLocked = false
	}

	if isLocked {
		return nil, ErrAccountLocked
	}

	// Check user status - only verified users can log in
	if user.Status != "verified" {
		if user.Status == "pending" {
			return nil, ErrEmailNotVerified
		}
		if user.Status == "suspended" {
			return nil, ErrAccountSuspended
		}
		if user.Status == "soft_deleted" {
			return nil, ErrAccountDeleted
		}
		// Any other status
		return nil, ErrInvalidCredentials
	}

	// Verify password against hash
	err = s.policy.VerifyPassword(user.PasswordHash, req.Password)
	if err != nil {
		// Invalid password - record the failed attempt. A persistence error here
		// must not change the response: answering 500 for an existing user (while
		// an unknown user gets a clean 401) is a username-enumeration oracle, and
		// it hides a broken lockout subsystem behind a generic error.
		if incrementErr := s.handleFailedLogin(ctx, user.ID); incrementErr != nil && s.logger != nil {
			s.logger.Error("failed to record failed login attempt: %v", incrementErr)
		}
		return nil, ErrInvalidCredentials
	}

	// Rehash with Argon2id if the hash uses an outdated algorithm (e.g. bcrypt)
	if s.policy.NeedsRehash(user.PasswordHash) {
		newHash, rehashErr := s.policy.HashPassword(req.Password)
		if rehashErr != nil {
			if s.logger != nil {
				s.logger.Error("failed to rehash password: %v", rehashErr)
			}
		} else if updateErr := s.userRepo.UpdatePasswordByUserID(ctx, user.ID, newHash); updateErr != nil {
			if s.logger != nil {
				s.logger.Error("failed to update rehashed password: %v", updateErr)
			}
		}
	}

	// Successful login - update last login timestamp and reset failed attempt counter
	_ = s.userRepo.UpdateLastLoginAt(ctx, user.ID)
	err = s.failedLoginRepo.ResetAttempts(ctx, user.ID)
	if err != nil {
		// Log error but don't fail login - this is not critical
		// The counter will be cleaned up by the next failed login or background job
		return &LoginResult{
			ID:       user.ID,
			UserID:   fmt.Sprintf("%d", user.ID),
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		}, nil
	}

	// Return user info for token generation
	return &LoginResult{
		ID:       user.ID,
		UserID:   fmt.Sprintf("%d", user.ID),
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}, nil
}

// IsAccountLocked checks if an account is currently locked
func (s *LoginService) IsAccountLocked(ctx context.Context, identifier string) (bool, *time.Time, error) {
	// Get user by username or email
	user, err := s.findUser(ctx, identifier)
	if err != nil {
		return false, nil, fmt.Errorf("database error: %w", err)
	}

	// User not found
	if user == nil {
		return false, nil, nil
	}

	// Check if account is locked
	return s.failedLoginRepo.IsLocked(ctx, user.ID)
}

// GetFailedLoginAttempts returns the number of failed login attempts for a user
func (s *LoginService) GetFailedLoginAttempts(ctx context.Context, identifier string) (int, error) {
	// Get user by username or email
	user, err := s.findUser(ctx, identifier)
	if err != nil {
		return 0, fmt.Errorf("database error: %w", err)
	}

	// User not found
	if user == nil {
		return 0, nil
	}

	// Get failed attempt record
	attempt, err := s.failedLoginRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		if err == ErrFailedLoginAttemptNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get attempt record: %w", err)
	}

	return attempt.Attempts, nil
}

// NewLoginService creates a new login service
func NewLoginService(
	userRepo userStore,
	failedLoginRepo FailedLoginAttemptRepo,
	policy CredentialPolicy,
	logger kernel.Logger,
) *LoginService {
	return &LoginService{
		userRepo:        userRepo,
		failedLoginRepo: failedLoginRepo,
		policy:          policy,
		logger:          logger,
	}
}
