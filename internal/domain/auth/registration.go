package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/repository"
)

var (
	// ErrUsernameInvalid is returned when username format is invalid
	ErrUsernameInvalid = errors.New("username must be 1-50 characters and contain only letters, numbers, underscores, and hyphens")

	// ErrUsernameExists is returned when username already exists
	ErrUsernameExists = errors.New("username already exists")

	// ErrEmailExists is returned when email already exists
	ErrEmailExists = errors.New("email address already registered")

	// ErrRegistrationFailed is returned when registration fails
	ErrRegistrationFailed = errors.New("registration failed")

	// ErrRegistrationDisabled is returned when self-registration is disabled via
	// the [registration] config block (or, by legacy default, when the comment
	// system is off).
	ErrRegistrationDisabled = errors.New("self-registration is disabled")
)

// usernameRegex validates username format (alphanumeric, underscores, hyphens, 1-50 chars)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// ValidateUsername checks if a username has a valid format
func ValidateUsername(username string) error {
	if username == "" || !usernameRegex.MatchString(username) {
		return ErrUsernameInvalid
	}
	return nil
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResult contains the result of a successful registration
type RegisterResult struct {
	UserID  int
	Message string
}

// RegistrationOption configures a RegistrationService. The zero options
// reproduce the legacy behavior exactly: registration enabled iff the comment
// system is enabled, default role Commentator, pending status.
type RegistrationOption func(*registrationOptions)

type registrationOptions struct {
	enabled     *bool
	defaultRole string
}

// WithEnabled overrides the registration gate. When unset, registration is
// enabled iff comments are enabled (the legacy coupling).
func WithEnabled(enabled bool) RegistrationOption {
	return func(o *registrationOptions) {
		o.enabled = &enabled
	}
}

// WithDefaultRole sets the role assigned to new registrants. When unset, it
// defaults to the Commentator role.
func WithDefaultRole(roleName string) RegistrationOption {
	return func(o *registrationOptions) {
		o.defaultRole = roleName
	}
}

// RegistrationService handles user registration business logic
type RegistrationService struct {
	userRepo        repository.UserRepo
	commentsEnabled bool
	enabled         bool
	defaultRole     string
}

// RegistrationEnabled reports whether self-registration is currently allowed.
// Handlers call this BEFORE any repository lookup (e.g. the blocked-email
// check) so a disabled instance short-circuits without a DB query and without
// leaking whether a submitted email is blocked.
func (s *RegistrationService) RegistrationEnabled() bool {
	return s.enabled
}

// RegisterUser registers a new user with validation. The registered user gets
// the configured default role (Commentator unless overridden) and is always
// created in "pending" status: email verification is mandatory, so the account
// only becomes usable after the registrant proves their email address (and, in
// admin-approval mode, after an administrator approves the account).
func (s *RegistrationService) RegisterUser(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	if !s.enabled {
		return nil, ErrRegistrationDisabled
	}

	// Validate username format
	if err := ValidateUsername(req.Username); err != nil {
		return nil, err
	}

	// Validate email format
	if err := auth.ValidateEmail(req.Email); err != nil {
		return nil, err
	}

	// Validate password strength
	if err := auth.ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	// Check username uniqueness (case-insensitive)
	usernameExists, err := s.userRepo.CheckUsernameExists(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistrationFailed, err)
	}
	if usernameExists {
		return nil, ErrUsernameExists
	}

	// Check email uniqueness (case-insensitive)
	emailExists, err := s.userRepo.CheckEmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistrationFailed, err)
	}
	if emailExists {
		return nil, ErrEmailExists
	}

	// Hash password
	passwordHash, _ := auth.HashPassword(req.Password)

	user := &repository.User{
		Username:     req.Username,
		PasswordHash: passwordHash,
		Email:        req.Email,
		Name:         req.Name,
		Role:         s.defaultRole,
		Status:       "pending",
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("%w: failed to create user", ErrRegistrationFailed)
	}

	return &RegisterResult{
		UserID:  user.ID,
		Message: "Registration successful. Please check your email to verify your account.",
	}, nil
}

// NewRegistrationService creates a new registration service. Legacy behavior —
// registration enabled iff comments enabled, default role Commentator, pending
// status — is preserved unless overridden via RegistrationOptions.
func NewRegistrationService(userRepo repository.UserRepo, commentsEnabled bool, opts ...RegistrationOption) *RegistrationService {
	o := registrationOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	enabled := commentsEnabled
	if o.enabled != nil {
		enabled = *o.enabled
	}
	defaultRole := o.defaultRole
	if defaultRole == "" {
		defaultRole = constants.RoleCommentator
	}

	return &RegistrationService{
		userRepo:        userRepo,
		commentsEnabled: commentsEnabled,
		enabled:         enabled,
		defaultRole:     defaultRole,
	}
}