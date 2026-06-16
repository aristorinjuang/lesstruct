package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
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

// LoginService handles login business logic with account lockout
type LoginService struct {
	userRepo        repository.UserRepo
	failedLoginRepo repository.FailedLoginAttemptRepo
	logger          *util.Logger
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string
	Password string
}

// LoginResult represents the result of a successful login
type LoginResult struct {
	UserID   string
	Username string
	Email    string
	Role     string
}

// handleFailedLogin handles a failed login attempt and implements account lockout
func (s *LoginService) handleFailedLogin(ctx context.Context, userID int) error {
	// Get current failed attempt record
	attempt, err := s.failedLoginRepo.GetByUserID(ctx, userID)
	if err != nil && err != repository.ErrFailedLoginAttemptNotFound {
		return fmt.Errorf("failed to get attempt record: %w", err)
	}

	// If no previous attempts, this is the first one
	if err == repository.ErrFailedLoginAttemptNotFound {
		return s.failedLoginRepo.Create(ctx, &repository.FailedLoginAttempt{
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

	// Get user by username
	user, err := s.userRepo.GetUserByUsername(ctx, req.Username)
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
		return nil, fmt.Errorf("failed to check lock status: %w", err)
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
	err = auth.VerifyPassword(user.PasswordHash, req.Password)
	if err != nil {
		// Invalid password - increment failed attempt counter
		incrementErr := s.handleFailedLogin(ctx, user.ID)
		if incrementErr != nil {
			return nil, fmt.Errorf("failed to record failed attempt: %w", incrementErr)
		}
		return nil, ErrInvalidCredentials
	}

	// Rehash with Argon2id if the hash uses an outdated algorithm (e.g. bcrypt)
	if auth.NeedsRehash(user.PasswordHash) {
		newHash, rehashErr := auth.HashPassword(req.Password)
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
			UserID:   fmt.Sprintf("%d", user.ID),
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		}, nil
	}

	// Return user info for token generation
	return &LoginResult{
		UserID:   fmt.Sprintf("%d", user.ID),
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}, nil
}

// IsAccountLocked checks if an account is currently locked
func (s *LoginService) IsAccountLocked(ctx context.Context, username string) (bool, *time.Time, error) {
	// Get user by username
	user, err := s.userRepo.GetUserByUsername(ctx, username)
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
func (s *LoginService) GetFailedLoginAttempts(ctx context.Context, username string) (int, error) {
	// Get user by username
	user, err := s.userRepo.GetUserByUsername(ctx, username)
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
		if err == repository.ErrFailedLoginAttemptNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get attempt record: %w", err)
	}

	return attempt.Attempts, nil
}

// NewLoginService creates a new login service
func NewLoginService(
	userRepo repository.UserRepo,
	failedLoginRepo repository.FailedLoginAttemptRepo,
	logger *util.Logger,
) *LoginService {
	return &LoginService{
		userRepo:        userRepo,
		failedLoginRepo: failedLoginRepo,
		logger:          logger,
	}
}
