package auth

import (
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
)

// FirstLoginService manages first-login setup state by deriving it from the database.
// Setup is considered complete when the admin password differs from the default.
type FirstLoginService struct{}

// IsSetupComplete returns whether first-login setup is complete by verifying
// the current admin password hash against the default password. Unlike a naive
// string comparison, this correctly handles salted hashes (Argon2id, bcrypt)
// where two hashes of the same password are never string-equal.
func (f *FirstLoginService) IsSetupComplete(currentAdminHash string) bool {
	return currentAdminHash != "" &&
		auth.VerifyPassword(currentAdminHash, constants.DefaultPassword) != nil
}

// NewFirstLoginService creates a new first-login service.
func NewFirstLoginService() *FirstLoginService {
	return &FirstLoginService{}
}
