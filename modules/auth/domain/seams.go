package auth

import (
	"context"
	"errors"

	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// DefaultUsername is the bootstrap admin login. It mirrors
// internal/constants.DefaultUsername (the frozen bootstrap vocabulary);
// the module owns its first-login comparisons.
const DefaultUsername = "admin"

// DefaultPassword is the bootstrap admin password compared (via hash
// verification, never string equality) during first-login setup detection.
const DefaultPassword = "admin"

// DefaultRole is the bootstrap admin role.
const DefaultRole = "Admin"

// DefaultAdminID is the stable identifier reported for the bootstrap admin.
const DefaultAdminID = "default-admin-001"

// RoleCommentator is the default role assigned to self-service registrants.
// It mirrors internal/constants.RoleCommentator (the frozen role
// vocabulary); the module owns its registration default.
const RoleCommentator = "Commentator"

// userStore is the user-repository subset the auth services need. The core
// user repository satisfies it structurally.
type userStore interface {
	GetUserByUsername(ctx context.Context, username string) (*userdomain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*userdomain.User, error)
	CreateUser(ctx context.Context, user *userdomain.User) error
	CheckUsernameExists(ctx context.Context, username string) (bool, error)
	CheckEmailExists(ctx context.Context, email string) (bool, error)
	SetEmailVerified(ctx context.Context, userID int, emailVerified bool) error
	UpdateUserStatus(ctx context.Context, userID int, status string) error
	UpdatePasswordByUserID(ctx context.Context, userID int, newPasswordHash string) error
	UpdateLastLoginAt(ctx context.Context, userID int) error
}

// CredentialPolicy validates credentials, hashes passwords, and reports
// hash staleness. The platform owns the policy (internal/auth rules plus
// the registration username rule); the module declares only the methods
// its services call.
type CredentialPolicy interface {
	ValidateEmail(email string) error
	ValidateUsername(username string) error
	ValidatePassword(password string) error
	GeneratePassword(length int) (string, error)
	HashPassword(password string) (string, error)
	VerifyPassword(hash, password string) error
	NeedsRehash(hash string) bool
}

var (
	// ErrInvalidEmail is returned when an email address fails validation.
	// Its message mirrors internal/auth.ErrEmailInvalid byte-for-byte: the
	// first-login handler surfaces policy errors on the wire, so the text
	// is part of the frozen contract. The adapter test pins the equality.
	ErrInvalidEmail = errors.New("please enter a valid email address")
	// ErrInvalidPassword is returned when a password fails validation.
	// Its message mirrors internal/auth.ErrPasswordInvalid byte-for-byte
	// for the same wire-text reason.
	ErrInvalidPassword = errors.New("password must be at least 12 characters with mixed case, numbers, and special characters")
)
