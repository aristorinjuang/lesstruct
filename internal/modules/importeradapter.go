package modules

import (
	"context"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	wordpress "github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// authorStore is the user-repository subset the WordPress importer needs.
// repository.UserRepo satisfies it; the narrow shape keeps the adapter
// decoupled from the full user-management surface.
type authorStore interface {
	GetUserByUsername(ctx context.Context, username string) (*userdomain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*userdomain.User, error)
	CreateUser(ctx context.Context, user *userdomain.User) error
}

// invalidEmailError carries a platform email-syntax rejection across the
// provisioner seam. Its message is the platform error verbatim (so import
// job output stays byte-identical to the pre-module text) while errors.Is
// still matches the module's ErrInvalidEmail sentinel for the resolver's
// legacy "invalid email" mapping.
type invalidEmailError struct {
	err error
}

func (e *invalidEmailError) Error() string {
	return e.err.Error()
}

func (e *invalidEmailError) Is(target error) bool {
	return target == wordpress.ErrInvalidEmail
}

// WordPressUserProvisioner adapts the user repository to the importers
// module's UserProvisioner seam. Account policy stays platform-side: email
// syntax is enforced with auth.ValidateEmail (mapped to the module's
// ErrInvalidEmail sentinel so job output keeps the legacy "invalid email"
// failure), passwords are generated and hashed here, and provisioned
// accounts are Contributors with verified status. It lives in core — the
// only layer that may import both sides.
type WordPressUserProvisioner struct {
	users authorStore
}

// FindByUsername returns the author account for username, or (nil, nil) when
// no account exists.
func (p *WordPressUserProvisioner) FindByUsername(ctx context.Context, username string) (*wordpress.AuthorAccount, error) {
	user, err := p.users.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &wordpress.AuthorAccount{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}

// FindByEmail returns the author account for email, or (nil, nil) when no
// account exists.
func (p *WordPressUserProvisioner) FindByEmail(ctx context.Context, email string) (*wordpress.AuthorAccount, error) {
	user, err := p.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &wordpress.AuthorAccount{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}

// CreateAuthor provisions a Contributor with verified status and a
// platform-generated password, returning the new account with its ID
// populated.
func (p *WordPressUserProvisioner) CreateAuthor(ctx context.Context, username, email, name string) (*wordpress.AuthorAccount, error) {
	if err := auth.ValidateEmail(email); err != nil {
		return nil, &invalidEmailError{err: err}
	}

	plainPassword, err := auth.GeneratePassword(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	passwordHash, err := auth.HashPassword(plainPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := &userdomain.User{
		Username:     username,
		PasswordHash: passwordHash,
		Email:        email,
		Name:         name,
		Role:         "Contributor",
		Status:       "verified",
	}
	if err := p.users.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &wordpress.AuthorAccount{
		ID:       newUser.ID,
		Username: username,
		Email:    email,
	}, nil
}

// NewWordPressUserProvisioner creates a WordPressUserProvisioner.
func NewWordPressUserProvisioner(users authorStore) *WordPressUserProvisioner {
	return &WordPressUserProvisioner{
		users: users,
	}
}
