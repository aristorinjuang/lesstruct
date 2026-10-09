package user

import (
	"context"
)

// RoleCommentator is the comment-system role. Admins may assign it only
// when comments are enabled. It mirrors internal/constants.RoleCommentator
// (the frozen role vocabulary); the module owns its assignment checks.
const RoleCommentator = "Commentator"

// CredentialPolicy validates credentials and hashes passwords. The platform
// owns the policy (internal/auth rules plus the registration username rule);
// the module declares only the methods its services call.
type CredentialPolicy interface {
	ValidateEmail(email string) error
	ValidateUsername(username string) error
	ValidatePassword(password string) error
	GeneratePassword(length int) (string, error)
	HashPassword(password string) (string, error)
	VerifyPassword(hash, password string) error
}

// Mailer sends the account emails the users domain triggers. It carries
// exactly the sends the services perform; the core email service satisfies
// it structurally.
type Mailer interface {
	SendEmailUpdateVerificationEmail(ctx context.Context, to, username, token string) error
	SendDataExportNotificationEmail(ctx context.Context, to, username string) error
	SendAccountDeletedNotification(ctx context.Context, to, username string) error
}

// RoleAssigner reports whether an admin may assign a role. The core role
// service satisfies it structurally.
type RoleAssigner interface {
	IsAssignable(role string) bool
}

// discardLogger drops log lines. It backs nil-logger fallbacks so services
// are safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}
