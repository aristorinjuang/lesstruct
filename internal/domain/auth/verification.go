package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

var (
	// ErrTokenInvalid is returned when verification token is invalid
	ErrTokenInvalid = errors.New("invalid verification token")

	// ErrTokenExpired is returned when verification token has expired
	ErrTokenExpired = errors.New("verification token has expired")

	// ErrVerificationFailed is returned when email verification fails
	ErrVerificationFailed = errors.New("email verification failed")
)

// VerificationOption configures a VerificationService. The zero options
// reproduce the legacy behavior exactly: a successful verification activates
// the account immediately.
type VerificationOption func(*verificationOptions)

type verificationOptions struct {
	adminApprovalRequired bool
}

// WithAdminApprovalRequired makes email verification a proof-of-ownership step
// only: the email address is recorded as verified, but the account stays
// "pending" until an administrator approves it. Without it, verification
// activates the account (legacy behavior).
func WithAdminApprovalRequired() VerificationOption {
	return func(o *verificationOptions) {
		o.adminApprovalRequired = true
	}
}

// VerificationResult contains the result of a successful verification
type VerificationResult struct {
	Success bool
	Message string
	UserID  int
	// AwaitingApproval reports that the email address was proven but the
	// account stays pending until an administrator approves it.
	AwaitingApproval bool
}

// CreateTokenResult contains the result of token creation
type CreateTokenResult struct {
	Token  string
	UserID int
}

// VerificationService handles email verification business logic
type VerificationService struct {
	userRepo              repository.UserRepo
	tokenRepo             repository.VerificationTokenRepo
	expiration            time.Duration
	adminApprovalRequired bool
}

// CreateVerificationToken creates a new verification token for a user
func (s *VerificationService) CreateVerificationToken(ctx context.Context, userID int) (*CreateTokenResult, error) {
	// Generate secure random token
	token, _ := repository.GenerateToken()

	// Hash token for storage
	tokenHash := repository.HashToken(token)

	// Create expiration timestamp (24 hours from now)
	expiresAt := time.Now().Add(s.expiration)

	// Delete any existing tokens for this user
	if err := s.tokenRepo.DeleteUserTokens(ctx, userID); err != nil {
		return nil, fmt.Errorf("%w: failed to delete old tokens", ErrVerificationFailed)
	}

	// Store new token
	verificationToken := &repository.VerificationToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := s.tokenRepo.CreateToken(ctx, verificationToken); err != nil {
		return nil, fmt.Errorf("%w: failed to create token", ErrVerificationFailed)
	}

	return &CreateTokenResult{
		Token:  token,
		UserID: userID,
	}, nil
}

// VerifyEmail verifies a user's email using a token
func (s *VerificationService) VerifyEmail(ctx context.Context, token string) (*VerificationResult, error) {
	// Hash token for lookup
	tokenHash := repository.HashToken(token)

	// Find valid token
	verificationToken, err := s.tokenRepo.FindValidToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to lookup token", ErrVerificationFailed)
	}

	if verificationToken == nil {
		return nil, ErrTokenInvalid
	}

	// Check if token is expired
	if time.Now().After(verificationToken.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Record the email proof first: in both modes the address is now verified.
	if err := s.userRepo.SetEmailVerified(ctx, verificationToken.UserID, true); err != nil {
		return nil, fmt.Errorf("%w: failed to mark email verified", ErrVerificationFailed)
	}

	// In admin-approval mode the account stays "pending" until an administrator
	// approves it; the email link only proves ownership of the address.
	if s.adminApprovalRequired {
		result := &VerificationResult{
			Success:          true,
			Message:          "Email verified. An administrator will activate your account.",
			UserID:           verificationToken.UserID,
			AwaitingApproval: true,
		}

		// Delete token after successful verification
		_ = s.tokenRepo.DeleteUserTokens(ctx, verificationToken.UserID)
		// Ignore deletion error - verification is already successful

		return result, nil
	}

	// Update user status to "verified"
	if err := s.userRepo.UpdateUserStatus(ctx, verificationToken.UserID, "verified"); err != nil {
		return nil, fmt.Errorf("%w: failed to update user status", ErrVerificationFailed)
	}

	result := &VerificationResult{
		Success: true,
		Message: "Email verified successfully",
		UserID:  verificationToken.UserID,
	}

	// Delete token after successful verification
	_ = s.tokenRepo.DeleteUserTokens(ctx, verificationToken.UserID)
	// Ignore deletion error - verification is already successful

	return result, nil
}

// CleanupExpiredTokens removes expired tokens from the database
func (s *VerificationService) CleanupExpiredTokens(ctx context.Context) error {
	return s.tokenRepo.DeleteExpiredTokens(ctx)
}

// NewVerificationService creates a new verification service. Legacy behavior —
// verification activates the account — is preserved unless overridden via
// VerificationOptions.
func NewVerificationService(
	userRepo repository.UserRepo,
	tokenRepo repository.VerificationTokenRepo,
	expirationHours int,
	opts ...VerificationOption,
) *VerificationService {
	o := verificationOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	return &VerificationService{
		userRepo:              userRepo,
		tokenRepo:             tokenRepo,
		expiration:            time.Duration(expirationHours) * time.Hour,
		adminApprovalRequired: o.adminApprovalRequired,
	}
}
