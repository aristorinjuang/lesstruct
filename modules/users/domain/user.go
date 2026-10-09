package user

import (
	"errors"
	"time"
)

var (
	// ErrAdminNotFound is returned when no admin user exists yet (fresh
	// install before first-login setup).
	ErrAdminNotFound = errors.New("admin user not found")
	// ErrSetupAlreadyCompleted is returned when first-login setup races:
	// another request completed it first.
	ErrSetupAlreadyCompleted = errors.New("first-login setup has already been completed")
)

// User represents a user in the system
type User struct {
	ID             int            `json:"id"`
	Username       string         `json:"username"`
	PasswordHash   string         `json:"-"`
	Email          string         `json:"email"`
	Name           string         `json:"name,omitempty"`
	Role           string         `json:"role"`
	Status         string         `json:"-"`
	EmailVerified  bool           `json:"-"`
	ProfilePicture string         `json:"profilePicture,omitempty"`
	LastLoginAt    *string        `json:"lastLoginAt,omitempty"`
	CustomFields   map[string]any `json:"customFields,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}
