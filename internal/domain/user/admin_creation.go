package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	authdomain "github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/aristorinjuang/lesstruct/internal/repository"
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

// allowedAdminRoles defines which roles an admin can assign
var allowedAdminRoles = map[string]bool{
	"Admin":       true,
	"Contributor": true,
	"Commentator": true,
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
	User          *repository.User
	PlainPassword string
}

// AdminCreateUserRepo defines the repository methods needed for admin user creation
type AdminCreateUserRepo interface {
	CheckUsernameExists(ctx context.Context, username string) (bool, error)
	CheckEmailExists(ctx context.Context, email string) (bool, error)
	CreateUser(ctx context.Context, user *repository.User) error
}

// AdminCreateUserService handles admin-initiated user creation
type AdminCreateUserService struct {
	userRepo        AdminCreateUserRepo
	blockedEmailRepo BlockedEmailRepo
}

// CreateUser creates a new user with verified status and an auto-generated password
func (s *AdminCreateUserService) CreateUser(ctx context.Context, req AdminCreateUserRequest) (*AdminCreateUserResult, error) {
	// Validate username format
	if err := authdomain.ValidateUsername(req.Username); err != nil {
		return nil, ErrUsernameInvalid
	}

	// Validate email format
	if err := auth.ValidateEmail(req.Email); err != nil {
		return nil, ErrEmailInvalid
	}

	// Validate role
	if !allowedAdminRoles[req.Role] {
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
	plainPassword, err := auth.GeneratePassword(16)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate password", ErrAdminCreateFailed)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(plainPassword)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to hash password", ErrAdminCreateFailed)
	}

	// Create user with verified status (bypasses email verification)
	displayName := req.Name
	if displayName == "" {
		displayName = req.Username
	}

	newUser := &repository.User{
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

// NewAdminCreateUserService creates a new admin create user service
func NewAdminCreateUserService(userRepo AdminCreateUserRepo, blockedEmailRepo BlockedEmailRepo) *AdminCreateUserService {
	return &AdminCreateUserService{
		userRepo:        userRepo,
		blockedEmailRepo: blockedEmailRepo,
	}
}
