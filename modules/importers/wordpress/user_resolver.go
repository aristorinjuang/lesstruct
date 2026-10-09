package wordpress

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aristorinjuang/lesstruct/kernel"
)

var (
	// ErrInvalidEmail is returned by UserProvisioner.CreateAuthor when the author
	// email fails the platform's syntax rules. The resolver maps it to the legacy
	// "invalid email" failure so import job output is unchanged.
	ErrInvalidEmail = errors.New("invalid author email")

	invalidUsernameChar = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	consecutiveHyphens  = regexp.MustCompile(`-{2,}`)
)

// sanitizeUsername converts a WordPress author login into a username that
// satisfies Lesstruct's rule of ^[a-zA-Z0-9_-]{1,50}$. Any character outside
// that set (spaces, dots, etc.) is replaced with a hyphen, consecutive hyphens
// are collapsed, and the result is trimmed and truncated to 50 characters.
func sanitizeUsername(login string) string {
	username := invalidUsernameChar.ReplaceAllString(login, "-")
	username = consecutiveHyphens.ReplaceAllString(username, "-")
	username = strings.Trim(username, "-")
	if len(username) > 50 {
		username = strings.Trim(username[:50], "-")
	}
	return username
}

// AuthorAccount is the user projection the importer needs: identity for
// reuse. Password material never crosses the seam — password generation,
// hashing, and role/status assignment are platform-owned.
type AuthorAccount struct {
	ID       int
	Username string
	Email    string
}

// UserProvisioner finds or creates the Lesstruct account for an imported
// author. The finders return (nil, nil) when no account exists. CreateAuthor
// provisions the account with the platform's password policy and email syntax
// rules, returning the new account with its ID populated.
type UserProvisioner interface {
	FindByUsername(ctx context.Context, username string) (*AuthorAccount, error)
	FindByEmail(ctx context.Context, email string) (*AuthorAccount, error)
	CreateAuthor(ctx context.Context, username, email, name string) (*AuthorAccount, error)
}

// UserResolver resolves WordPress authors to Lesstruct user IDs. If a user
// already exists (by username or email) it is reused; otherwise a new account
// is provisioned through the platform-owned UserProvisioner.
type UserResolver struct {
	provisioner UserProvisioner
	logger      kernel.Logger
}

// ResolveOrCreate returns the Lesstruct userID for the given WordPress author.
// The created flag is true when a new user was created.
func (r *UserResolver) ResolveOrCreate(
	ctx context.Context,
	login,
	email,
	displayName string,
) (int, bool, error) {
	username := sanitizeUsername(login)
	if username == "" {
		return 0, false, fmt.Errorf("username could not be derived from login %q", login)
	}

	existing, err := r.provisioner.FindByUsername(ctx, username)
	if err != nil {
		return 0, false, fmt.Errorf("failed to look up user by username %q: %w", username, err)
	}
	if existing != nil {
		return existing.ID, false, nil
	}

	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return 0, false, fmt.Errorf("author %q has no email address", login)
	}
	existing, err = r.provisioner.FindByEmail(ctx, trimmedEmail)
	if err != nil {
		return 0, false, fmt.Errorf("failed to look up user by email %q: %w", trimmedEmail, err)
	}
	if existing != nil {
		return existing.ID, false, nil
	}

	name := displayName
	if name == "" {
		name = login
	}

	created, err := r.provisioner.CreateAuthor(ctx, username, trimmedEmail, name)
	if err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			return 0, false, fmt.Errorf("invalid email %q: %w", trimmedEmail, err)
		}
		return 0, false, fmt.Errorf("failed to create user for author %q: %w", login, err)
	}

	if r.logger != nil {
		r.logger.Info(
			"WordPress import: created user %q (ID: %d) for author %q",
			username,
			created.ID,
			login,
		)
	}

	return created.ID, true, nil
}

// NewUserResolver creates a UserResolver.
func NewUserResolver(provisioner UserProvisioner, logger kernel.Logger) *UserResolver {
	return &UserResolver{
		provisioner: provisioner,
		logger:      logger,
	}
}
