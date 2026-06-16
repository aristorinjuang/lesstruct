package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/aristorinjuang/lesstruct/internal/auth"
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

// RegistrationService handles user registration business logic
type RegistrationService struct {
	userRepo repository.UserRepo
}

// RegisterUser registers a new user with validation
func (s *RegistrationService) RegisterUser(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
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

	// Create user with pending status
	user := &repository.User{
		Username:     req.Username,
		PasswordHash: passwordHash,
		Email:        req.Email,
		Name:         req.Name,
		Role:         "Commentator",
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

// NewRegistrationService creates a new registration service
func NewRegistrationService(userRepo repository.UserRepo) *RegistrationService {
	return &RegistrationService{
		userRepo: userRepo,
	}
}
