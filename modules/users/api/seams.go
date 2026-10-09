package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aristorinjuang/lesstruct/kernel"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// principalID parses the principal's decimal ID for repository calls,
// mirroring identity.IntID (fail-closed with the kernel sentinel).
func principalID(principal kernel.Principal) (int, error) {
	id, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", kernel.ErrInvalidPrincipalID, principal.ID)
	}
	return id, nil
}

// discardLogger drops log lines. It backs nil-logger fallbacks so handlers
// are safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

var _ = kernel.Logger(discardLogger{})

// UserStore is the user-repository subset the users handlers need.
type UserStore interface {
	GetUserByID(ctx context.Context, userID int) (*userdomain.User, error)
}

// SoftDeletedContent mirrors the soft-deleted content record without
// database null types: the core adapter converts them at the boundary.
type SoftDeletedContent struct {
	ID          int
	ContentType string
	ContentID   int
	DeletedAt   string
	DeletedBy   int
	Reason      string
	HasReason   bool
}

// SoftDeleteStore is the soft-delete repository subset the user management
// handler needs. The core soft-delete repository satisfies it via the
// core-side adapter (null-type conversion lives there).
type SoftDeleteStore interface {
	GetSoftDeletedContentByUser(ctx context.Context, userID int) ([]*SoftDeletedContent, error)
	GetSoftDeletedContentByID(ctx context.Context, id int) (*SoftDeletedContent, error)
	RestoreContent(ctx context.Context, contentType string, contentID int) error
}

// Mailer sends the account emails user management triggers. It carries
// exactly the sends the handlers perform; the core email service satisfies
// it structurally.
type Mailer interface {
	SendUserApprovedEmail(ctx context.Context, to, username string) error
	SendUserRejectedEmail(ctx context.Context, to, username string) error
	SendUserSuspendedEmail(ctx context.Context, to, username, reason string) error
	SendUserUnsuspendedEmail(ctx context.Context, to, username string) error
	SendUserSoftDeletedEmail(ctx context.Context, to, username, reason string) error
	SendAccountCreatedEmail(ctx context.Context, to, username, plainPassword string) error
}
