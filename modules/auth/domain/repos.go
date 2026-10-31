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

// MFAChallengeRepo defines the interface for login MFA challenge repository
// operations. Challenges hold only hashed tokens and codes; successful
// verification deletes the row so a challenge is single-use.
type MFAChallengeRepo interface {
	Create(ctx context.Context, challenge *MFAChallenge) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*MFAChallenge, error)
	// IncrementAttempts spends one attempt only while the challenge is below
	// maxAttempts. It reports whether the attempt was spent.
	IncrementAttempts(ctx context.Context, id, maxAttempts int) (bool, error)
	// Consume deletes the challenge and reports whether this caller removed
	// it. A concurrent verify that loses the race gets false.
	Consume(ctx context.Context, id int) (bool, error)
	Delete(ctx context.Context, id int) error
	DeleteUserChallenges(ctx context.Context, userID int) error
	DeleteExpired(ctx context.Context) error
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
