package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrResetTokenInvalid is returned when password reset token is invalid
	ErrResetTokenInvalid = errors.New("invalid password reset token")

	// ErrResetTokenExpired is returned when password reset token has expired
	ErrResetTokenExpired = errors.New("password reset token has expired")

	// ErrResetFailed is returned when password reset fails
	ErrResetFailed = errors.New("password reset failed")
)

// PasswordResetResult contains the result of a successful password reset
type PasswordResetResult struct {
	Success bool
	Message string
	UserID  int
}

// RequestPasswordResetResult contains the result of a password reset request
type RequestPasswordResetResult struct {
	Token  string
	UserID int
	Email  string
	Name   string
}

// PasswordResetService handles password reset business logic
type PasswordResetService struct {
	userRepo   userStore
	tokenRepo  PasswordResetTokenRepo
	policy     CredentialPolicy
	expiration time.Duration
}

// RequestPasswordReset creates a password reset token for a user
func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, email string) (*RequestPasswordResetResult, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to lookup user", ErrResetFailed)
	}

	if user == nil {
		return nil, nil
	}

	if user.Status != "verified" {
		return nil, nil
	}

	token, err := GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate token", ErrResetFailed)
	}
	tokenHash := HashToken(token)
	expiresAt := time.Now().Add(s.expiration)

	if err := s.tokenRepo.DeleteUserTokens(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("%w: failed to delete old tokens", ErrResetFailed)
	}

	resetToken := &PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := s.tokenRepo.CreateToken(ctx, resetToken); err != nil {
		return nil, fmt.Errorf("%w: failed to create token", ErrResetFailed)
	}

	return &RequestPasswordResetResult{
		Token:  token,
		UserID: user.ID,
		Email:  user.Email,
		Name:   user.Name,
	}, nil
}

// ResetPassword verifies the token and updates the user's password
func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) (*PasswordResetResult, error) {
	if err := s.policy.ValidatePassword(newPassword); err != nil {
		return nil, err
	}

	tokenHash := HashToken(token)
	resetToken, err := s.tokenRepo.FindValidToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to lookup token", ErrResetFailed)
	}

	if resetToken == nil {
		return nil, ErrResetTokenInvalid
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return nil, ErrResetTokenExpired
	}

	passwordHash, err := s.policy.HashPassword(newPassword)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to hash password", ErrResetFailed)
	}

	if err := s.userRepo.UpdatePasswordByUserID(ctx, resetToken.UserID, passwordHash); err != nil {
		return nil, fmt.Errorf("%w: failed to update password", ErrResetFailed)
	}

	_ = s.tokenRepo.DeleteUserTokens(ctx, resetToken.UserID)

	return &PasswordResetResult{
		Success: true,
		Message: "Password reset successfully",
		UserID:  resetToken.UserID,
	}, nil
}

// NewPasswordResetService creates a new password reset service
func NewPasswordResetService(
	userRepo userStore,
	tokenRepo PasswordResetTokenRepo,
	policy CredentialPolicy,
	expirationHours int,
) *PasswordResetService {
	return &PasswordResetService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		policy:     policy,
		expiration: time.Duration(expirationHours) * time.Hour,
	}
}
