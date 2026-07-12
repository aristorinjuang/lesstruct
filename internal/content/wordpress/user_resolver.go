package wordpress

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

var (
	invalidUsernameChar = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	consecutiveHyphens  = regexp.MustCompile(`-{2,}`)
)

// userResolverRepo is the repository subset needed to resolve or create users
// during import.
type userResolverRepo interface {
	GetUserByUsername(ctx context.Context, username string) (*repository.User, error)
	GetUserByEmail(ctx context.Context, email string) (*repository.User, error)
	CreateUser(ctx context.Context, user *repository.User) error
}

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

// UserResolver resolves WordPress authors to Lesstruct user IDs. If a user
// already exists (by username or email) it is reused; otherwise a new
// Contributor with a random password and verified status is created.
type UserResolver struct {
	repo   userResolverRepo
	logger *util.Logger
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

	existing, err := r.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return 0, false, fmt.Errorf("failed to look up user by username %q: %w", username, err)
	}
	if existing != nil {
		return existing.ID, false, nil
	}

	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail != "" {
		existing, err = r.repo.GetUserByEmail(ctx, trimmedEmail)
		if err != nil {
			return 0, false, fmt.Errorf("failed to look up user by email %q: %w", trimmedEmail, err)
		}
		if existing != nil {
			return existing.ID, false, nil
		}
		if err := auth.ValidateEmail(trimmedEmail); err != nil {
			return 0, false, fmt.Errorf("invalid email %q: %w", trimmedEmail, err)
		}
	} else {
		return 0, false, fmt.Errorf("author %q has no email address", login)
	}

	plainPassword, err := auth.GeneratePassword(16)
	if err != nil {
		return 0, false, fmt.Errorf("failed to generate password for author %q: %w", login, err)
	}

	passwordHash, err := auth.HashPassword(plainPassword)
	if err != nil {
		return 0, false, fmt.Errorf("failed to hash password for author %q: %w", login, err)
	}

	name := displayName
	if name == "" {
		name = login
	}

	newUser := &repository.User{
		Username:     username,
		PasswordHash: passwordHash,
		Email:        trimmedEmail,
		Name:         name,
		Role:         "Contributor",
		Status:       "verified",
	}

	if err := r.repo.CreateUser(ctx, newUser); err != nil {
		return 0, false, fmt.Errorf("failed to create user for author %q: %w", login, err)
	}

	if r.logger != nil {
		r.logger.Info(
			"WordPress import: created user %q (ID: %d) for author %q",
			username,
			newUser.ID,
			login,
		)
	}

	return newUser.ID, true, nil
}

// NewUserResolver creates a UserResolver.
func NewUserResolver(repo userResolverRepo, logger *util.Logger) *UserResolver {
	return &UserResolver{
		repo:   repo,
		logger: logger,
	}
}
