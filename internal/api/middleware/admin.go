package middleware

import (
	"net/http"
)

// AdminMiddleware represents admin authorization middleware
type AdminMiddleware struct {
	authMiddleware *AuthMiddleware
}

// AdminOnly validates JWT token and checks if user has Admin role
// This is a convenience wrapper around RequireRole("Admin")
func (m *AdminMiddleware) AdminOnly(next http.Handler) http.Handler {
	return m.authMiddleware.RequireRole(RoleAdmin)(next)
}

// ModerationOnly validates JWT token and checks if user has Admin role
func (m *AdminMiddleware) ModerationOnly(next http.Handler) http.Handler {
	return m.authMiddleware.RequireRole(RoleAdmin)(next)
}

// NewAdminMiddleware creates a new admin authorization middleware
func NewAdminMiddleware(authMiddleware *AuthMiddleware) *AdminMiddleware {
	return &AdminMiddleware{
		authMiddleware: authMiddleware,
	}
}
