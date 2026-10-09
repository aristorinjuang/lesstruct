package auth

import (
	"errors"
	"fmt"
)

// UserInfo contains user information for JWT token generation
type UserInfo struct {
	ID       string
	Username string
	Role     string
}

// String returns a string representation of user info
func (u *UserInfo) String() string {
	return fmt.Sprintf("User{ID: %s, Username: %s, Role: %s}", u.ID, u.Username, u.Role)
}

// AuthService handles authentication business logic
type AuthService struct {
	passwordHash string
	policy       CredentialPolicy
}

// Login validates credentials and returns user info if valid
func (a *AuthService) Login(username, password string) (*UserInfo, error) {
	// For this story, we only support default credentials
	if username != DefaultUsername {
		return nil, errors.New("invalid username or password")
	}

	// Verify password against hash
	err := a.policy.VerifyPassword(a.passwordHash, password)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	// Return user info for token generation
	return &UserInfo{
		ID:       DefaultAdminID,
		Username: DefaultUsername,
		Role:     DefaultRole,
	}, nil
}

// NewAuthService creates a new auth service with default admin credentials
func NewAuthService(passwordHash string, policy CredentialPolicy) *AuthService {
	return &AuthService{
		passwordHash: passwordHash,
		policy:       policy,
	}
}
