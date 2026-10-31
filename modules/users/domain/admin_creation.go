package user

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrUsernameInvalid is returned when username format is invalid
	ErrUsernameInvalid = errors.New("username must be 1-50 characters and contain only letters, numbers, underscores, and hyphens")

	// ErrEmailInvalid is returned when email format is invalid
	ErrEmailInvalid = errors.New("please enter a valid email address")

	// ErrUsernameExists is returned when username already exists
	ErrUsernameExists = errors.New("username already exists")

	// ErrEmailExists is returned when email already exists
	ErrEmailExists = errors.New("email address already registered")

	// ErrEmailBlocked is returned when email is blocked
	ErrEmailBlocked = errors.New("this email address has been blocked")

	// ErrInvalidRole is returned when role is not in the allowed list
	ErrInvalidRole = errors.New("invalid user role")

	// ErrAdminCreateFailed is returned when admin user creation fails
	ErrAdminCreateFailed = errors.New("admin user creation failed")
)

// allowedAdminRoles defines the legacy fallback role allowlist, used only when
// an AdminCreateUserService is constructed without a role service (e.g. in
// tests). Production always injects the config-driven role registry.
var allowedAdminRoles = map[string]bool{
	"Admin":       true,
	"Contributor": true,
	"Commentator": true,
}

// isAllowedRole reports whether an admin may assign the role. The Commentator
// role exists only for the comment system; when comments are disabled it
// cannot be assigned. Shared by the admin create-user and update-user paths so
// both stay consistent with the [comments] config toggle.
func isAllowedRole(role string, commentsEnabled bool) bool {
	if !allowedAdminRoles[role] {
		return false
	}
	if role == RoleCommentator && !commentsEnabled {
		return false
	}
	return true
}

// AdminCreateUserRequest represents the input for admin user creation
type AdminCreateUserRequest struct {
	Username     string
	Name         string
	Email        string
	Role         string
	CustomFields map[string]any
}

// AdminCreateUserResult contains the result of admin user creation
type AdminCreateUserResult struct {
	User          *User
	PlainPassword string
}

// AdminCreateUserRepo defines the repository methods needed for admin user creation
type AdminCreateUserRepo interface {
	CheckUsernameExists(ctx context.Context, username string) (bool, error)
	CheckEmailExists(ctx context.Context, email string) (bool, error)
	CreateUser(ctx context.Context, user *User) error
}

// AdminCreateUserOption configures an AdminCreateUserService. A role service
// makes the assignable-role allowlist config-driven; without one the legacy
// three-role allowlist applies.
type AdminCreateUserOption func(*AdminCreateUserService)

// WithRoleService attaches the config-driven role registry so admins can assign
// every registered role (built-ins plus any [[role]] entries).
func WithRoleService(rs RoleAssigner) AdminCreateUserOption {
	return func(s *AdminCreateUserService) {
		s.roleService = rs
	}
}

// WithRegistrationEnabled overrides the registration gate consulted for the
// Commentator role. When unset it defaults to the comments flag (legacy
// coupling); the composition root sets it from [registration].IsEnabled.
func WithRegistrationEnabled(enabled bool) AdminCreateUserOption {
	return func(s *AdminCreateUserService) {
		s.registrationEnabled = &enabled
	}
}

// AdminCreateUserService handles admin-initiated user creation
type AdminCreateUserService struct {
	userRepo            AdminCreateUserRepo
	blockedEmailRepo    BlockedEmailRepo
	policy              CredentialPolicy
	commentsEnabled     bool
	registrationEnabled *bool
	roleService         RoleAssigner
}

// registrationAllowsCommentator reports whether the Commentator role may be
// assigned: it requires the comment system and self-registration both
// enabled. An unset registration override defaults to the comments flag,
// preserving the legacy coupling.
func (s *AdminCreateUserService) registrationAllowsCommentator() bool {
	if s.registrationEnabled != nil {
		return *s.registrationEnabled
	}
	return s.commentsEnabled
}

// isAllowedRole reports whether an admin may assign the role. With a role
// service the allowlist is registry-driven; the Commentator role additionally
// requires the comment system and registration both enabled.
func (s *AdminCreateUserService) isAllowedRole(role string) bool {
	commentatorAllowed := s.commentsEnabled && s.registrationAllowsCommentator()
	if s.roleService != nil {
		if !s.roleService.IsAssignable(role) {
			return false
		}
		if role == RoleCommentator && !commentatorAllowed {
			return false
		}
		return true
	}
	if !allowedAdminRoles[role] {
		return false
	}
	if role == RoleCommentator && !commentatorAllowed {
		return false
	}
	return true
}

// CreateUser creates a new user with verified status and an auto-generated password
func (s *AdminCreateUserService) CreateUser(ctx context.Context, req AdminCreateUserRequest) (*AdminCreateUserResult, error) {
	// Validate username format
	if err := s.policy.ValidateUsername(req.Username); err != nil {
		return nil, ErrUsernameInvalid
	}

	// Validate email format
	if err := s.policy.ValidateEmail(req.Email); err != nil {
		return nil, ErrEmailInvalid
	}

	// Validate role
	if !s.isAllowedRole(req.Role) {
		return nil, ErrInvalidRole
	}

	// Check blocked email list
	blocked, err := s.blockedEmailRepo.IsEmailBlocked(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to check blocked emails", ErrAdminCreateFailed)
	}
	if blocked {
		return nil, ErrEmailBlocked
	}

	// Check username uniqueness
	usernameExists, err := s.userRepo.CheckUsernameExists(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to check username", ErrAdminCreateFailed)
	}
	if usernameExists {
		return nil, ErrUsernameExists
	}

	// Check email uniqueness
	emailExists, err := s.userRepo.CheckEmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to check email", ErrAdminCreateFailed)
	}
	if emailExists {
		return nil, ErrEmailExists
	}

	// Generate password
	plainPassword, err := s.policy.GeneratePassword(16)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate password", ErrAdminCreateFailed)
	}

	// Hash password
	passwordHash, err := s.policy.HashPassword(plainPassword)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to hash password", ErrAdminCreateFailed)
	}

	// Create user with verified status (bypasses email verification)
	displayName := req.Name
	if displayName == "" {
		displayName = req.Username
	}

	newUser := &User{
		Username:     req.Username,
		PasswordHash: passwordHash,
		Email:        req.Email,
		Name:         displayName,
		Role:         req.Role,
		Status:       "verified",
		CustomFields: req.CustomFields,
	}

	if err := s.userRepo.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("%w: failed to create user", ErrAdminCreateFailed)
	}

	return &AdminCreateUserResult{
		User:          newUser,
		PlainPassword: plainPassword,
	}, nil
}

// NewAdminCreateUserService creates a new admin create user service.
// commentsEnabled gates the Commentator role: when false, admins cannot assign
// it (the role exists only for the comment system).
func NewAdminCreateUserService(
	userRepo AdminCreateUserRepo,
	blockedEmailRepo BlockedEmailRepo,
	policy CredentialPolicy,
	commentsEnabled bool,
	opts ...AdminCreateUserOption,
) *AdminCreateUserService {
	s := &AdminCreateUserService{
		userRepo:         userRepo,
		blockedEmailRepo: blockedEmailRepo,
		policy:           policy,
		commentsEnabled:  commentsEnabled,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
