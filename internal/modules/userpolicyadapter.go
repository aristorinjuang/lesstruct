package modules

import (
	"github.com/aristorinjuang/lesstruct/internal/auth"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
)

// UserCredentialPolicy adapts the platform credential rules to the users
// module's CredentialPolicy seam: email/password/username syntax from
// internal/auth plus the registration username rule from the auth module.
// Password hashing uses the platform argon2id parameters, so hashes remain
// verifiable across the move. It lives in core — the only layer that may
// import both sides.
type UserCredentialPolicy struct{}

// ValidateEmail checks email format per platform rules, translating the
// platform error to the module sentinel (same message, matchable without
// importing internal packages).
func (UserCredentialPolicy) ValidateEmail(email string) error {
	if err := auth.ValidateEmail(email); err != nil {
		return authdomain.ErrInvalidEmail
	}
	return nil
}

// ValidateUsername checks username format per the registration rule.
func (UserCredentialPolicy) ValidateUsername(username string) error {
	return authdomain.ValidateUsername(username)
}

// ValidatePassword checks password strength per platform rules, translating
// the platform error to the module sentinel (same message, matchable
// without importing internal packages).
func (UserCredentialPolicy) ValidatePassword(password string) error {
	if err := auth.ValidatePassword(password); err != nil {
		return authdomain.ErrInvalidPassword
	}
	return nil
}

// GeneratePassword generates a random password of the given length.
func (UserCredentialPolicy) GeneratePassword(length int) (string, error) {
	return auth.GeneratePassword(length)
}

// HashPassword hashes a password with the platform parameters.
func (UserCredentialPolicy) HashPassword(password string) (string, error) {
	return auth.HashPassword(password)
}

// VerifyPassword verifies a password against its hash.
func (UserCredentialPolicy) VerifyPassword(hash, password string) error {
	return auth.VerifyPassword(hash, password)
}

// NeedsRehash reports whether a hash uses outdated parameters.
func (UserCredentialPolicy) NeedsRehash(hash string) bool {
	return auth.NeedsRehash(hash)
}

// NewUserCredentialPolicy creates a UserCredentialPolicy. The concrete
// value satisfies both the users and the auth CredentialPolicy seams.
func NewUserCredentialPolicy() UserCredentialPolicy {
	return UserCredentialPolicy{}
}
