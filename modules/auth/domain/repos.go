package auth

import (
	"context"
	"time"
)

// VerificationTokenRepo defines the interface for verification token repository operations
type VerificationTokenRepo interface {
	CreateToken(ctx context.Context, token *VerificationToken) error
	FindValidToken(ctx context.Context, tokenHash string) (*VerificationToken, error)
	DeleteUserTokens(ctx context.Context, userID int) error
	DeleteExpiredTokens(ctx context.Context) error
}

// PasswordResetTokenRepo defines the interface for password reset token repository operations
type PasswordResetTokenRepo interface {
	CreateToken(ctx context.Context, token *PasswordResetToken) error
	FindValidToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error)
	DeleteUserTokens(ctx context.Context, userID int) error
}

// FailedLoginAttemptRepo defines the interface for failed login attempt repository operations
type FailedLoginAttemptRepo interface {
	GetByUserID(ctx context.Context, userID int) (*FailedLoginAttempt, error)
	Create(ctx context.Context, attempt *FailedLoginAttempt) error
	IncrementAttempts(ctx context.Context, userID int) error
	LockAccount(ctx context.Context, userID int, lockoutDuration time.Duration) error
	ResetAttempts(ctx context.Context, userID int) error
	Delete(ctx context.Context, userID int) error
	IsLocked(ctx context.Context, userID int) (bool, *time.Time, error)
	UpdateLastEmailSent(ctx context.Context, userID int) error
	ShouldSendLockoutEmail(ctx context.Context, userID int, minInterval time.Duration) (bool, error)
}
