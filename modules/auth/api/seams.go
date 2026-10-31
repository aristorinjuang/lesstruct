package api

import (
	"context"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// TokenIssuer mints JWTs for authenticated sessions. The core JWT manager
// satisfies it structurally through the composition-root adapter;
// verification stays core-side (middleware).
type TokenIssuer interface {
	GenerateToken(userID, username, role string, tokenVersion int) (string, error)
	// ValidateTokenExpiry validates the token and reports its expiry so the
	// session cookie lifetime matches the token instead of duplicating the
	// TTL.
	ValidateTokenExpiry(token string) (time.Time, error)
}

// Mailer sends the account emails the auth flow triggers. It carries
// exactly the sends the handlers perform; the core email service satisfies
// it structurally.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, username, token string) error
	SendPasswordResetEmail(ctx context.Context, to, username, token string) error
	SendAccountLockoutEmail(ctx context.Context, to, username string) error
	SendRegistrationAttemptEmail(ctx context.Context, to, username string) error
}

// MFACodeDeliverer delivers the one-time login code to the second factor
// channel. The composition root picks the adapter: email whenever SMTP is
// configured, and a log-only sink only when DEV_MODE is on and SMTP is absent.
type MFACodeDeliverer interface {
	DeliverMFACode(ctx context.Context, to, username, code string) error
}

// NotificationRecorder records admin-facing notifications for auth events.
// It names intents, not storage types: the core adapter persists them
// through the notification repository.
type NotificationRecorder interface {
	RecordUserRegistered(ctx context.Context, username, email string) error
	RecordUserVerified(ctx context.Context, username, email string) error
}

// BlockedEmailChecker reports whether an email address is blocked from
// registering. The core blocked-email repository satisfies it structurally.
type BlockedEmailChecker interface {
	IsEmailBlocked(ctx context.Context, email string) (bool, error)
}

// discardLogger drops log lines. It backs nil-logger fallbacks so handlers
// are safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

var _ = kernel.Logger(discardLogger{})

// UserStore is the user-repository subset the auth handlers need.
type UserStore interface {
	GetUserByUsername(ctx context.Context, username string) (*userdomain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*userdomain.User, error)
	GetUserByID(ctx context.Context, userID int) (*userdomain.User, error)
	GetAdminUser(ctx context.Context) (*userdomain.User, error)
	UpdateAdminPasswordAndEmail(ctx context.Context, passwordHash, email, currentPasswordHash string) error
	// RevokeSessions bumps the stored token version so every previously
	// issued session for the user stops validating.
	RevokeSessions(ctx context.Context, userID int) error
}
