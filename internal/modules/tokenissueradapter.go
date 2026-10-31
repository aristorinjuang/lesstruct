package modules

import (
	"time"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	authapi "github.com/aristorinjuang/lesstruct/modules/auth/api"
)

var (
	_ authapi.TokenIssuer = (*JWTTokenIssuer)(nil)
)

// JWTTokenIssuer adapts the platform JWT manager to the auth module's
// TokenIssuer seam. It lives in core — the only layer that may import both
// sides — so the module never reaches into internal packages.
type JWTTokenIssuer struct {
	manager *auth.JWTManager
}

// GenerateToken mints a JWT for an authenticated session.
func (a *JWTTokenIssuer) GenerateToken(
	userID, username, role string,
	tokenVersion int,
) (string, error) {
	return a.manager.GenerateToken(userID, username, role, tokenVersion)
}

// ValidateTokenExpiry validates the token and reports its expiry so the
// session cookie lifetime matches the token instead of duplicating the TTL.
func (a *JWTTokenIssuer) ValidateTokenExpiry(token string) (time.Time, error) {
	claims, err := a.manager.ValidateToken(token)
	if err != nil {
		return time.Time{}, err
	}

	return claims.ExpiresAt.Time, nil
}

// NewJWTTokenIssuer creates a TokenIssuer backed by the platform JWT manager.
func NewJWTTokenIssuer(manager *auth.JWTManager) *JWTTokenIssuer {
	return &JWTTokenIssuer{
		manager: manager,
	}
}
