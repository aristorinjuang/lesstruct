package middleware

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
)

const (
	UserIDKey   contextKey = "user_id"
	UsernameKey contextKey = "username"
	RoleKey     contextKey = "role"
)

// Role constants for easy reference in middleware
const (
	RoleAdmin       = constants.RoleAdmin
	RoleContributor = constants.RoleContributor
	RoleCommentator = constants.RoleCommentator
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

var (
	ErrMissingToken = errors.New("missing authorization token")
	ErrInvalidToken = errors.New("invalid authorization token")
)

// newUserContext populates request context with user identity using the shared
// context keys. Both the JWT AuthMiddleware and the API-key APIKeyAuthMiddleware
// route through this so downstream ctxUser lookups and RBAC are auth-agnostic
// (architecture Important Gap #1). Values are strings (UserID is the decimal id).
func newUserContext(ctx context.Context, userID, username, role string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, UsernameKey, username)
	ctx = context.WithValue(ctx, RoleKey, role)
	return ctx
}

// UserContext represents user information stored in request context
type UserContext struct {
	UserID   string
	Username string
	Role     string
}

// AuthMiddleware represents authentication middleware
type AuthMiddleware struct {
	jwtManager *auth.JWTManager
}

// extractToken extracts the JWT token from the Authorization header
func (m *AuthMiddleware) extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingToken
	}

	// Check if header starts with "Bearer " (exactly one space after Bearer)
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", ErrInvalidToken
	}

	// Extract token and trim whitespace
	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)

	// Validate token is not empty after trimming
	if token == "" {
		return "", ErrInvalidToken
	}

	// Check for multiple Bearer prefixes (potential attack)
	if strings.Contains(strings.ToLower(token), "bearer ") {
		return "", ErrInvalidToken
	}

	return token, nil
}

// createUserContext now delegates to the shared helper.
func (m *AuthMiddleware) createUserContext(ctx context.Context, claims *auth.Claims) context.Context {
	return newUserContext(ctx, claims.UserID, claims.Username, claims.Role)
}

// hasRequiredRole checks if the user role is in the list of required roles
func (m *AuthMiddleware) hasRequiredRole(userRole string, requiredRoles []string) bool {
	return slices.Contains(requiredRoles, userRole)
}

// RequireAuth validates JWT token and adds user context to request
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		token, err := m.extractToken(r)
		if err != nil {
			response.Error(w, http.StatusUnauthorized, "MISSING_TOKEN", "Missing authorization token", nil)
			return
		}

		// Validate token
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			// Check if token is expired
			if strings.Contains(err.Error(), "expired") {
				response.Error(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Your session has expired. Please log in again", nil)
				return
			}

			// Invalid token
			response.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid authorization token", nil)
			return
		}

		// Add user context to request
		ctx := m.createUserContext(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth validates JWT token if present, but doesn't require it
// Adds user context if token is valid, otherwise continues without user context
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to extract token
		token, err := m.extractToken(r)
		if err != nil {
			// No token present, continue without user context
			next.ServeHTTP(w, r)
			return
		}

		// Try to validate token
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			// Invalid token, continue without user context
			next.ServeHTTP(w, r)
			return
		}

		// Add user context to request
		ctx := m.createUserContext(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole validates JWT token and checks if user has required role
func (m *AuthMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			token, err := m.extractToken(r)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "MISSING_TOKEN", "Missing authorization token", nil)
				return
			}

			// Validate token
			claims, err := m.jwtManager.ValidateToken(token)
			if err != nil {
				// Check if token is expired
				if strings.Contains(err.Error(), "expired") {
					response.Error(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Your session has expired. Please log in again", nil)
					return
				}

				// Invalid token
				response.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid authorization token", nil)
				return
			}

			// Check if user has required role
			if !m.hasRequiredRole(claims.Role, roles) {
				response.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "You do not have permission to access this resource", nil)
				return
			}

			// Add user context to request
			ctx := m.createUserContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtManager *auth.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
	}
}

// GetUserID retrieves the user ID from the request context
func GetUserID(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	return userID, ok
}

// GetUsername retrieves the username from the request context
func GetUsername(r *http.Request) (string, bool) {
	username, ok := r.Context().Value(UsernameKey).(string)
	return username, ok
}

// GetRole retrieves the role from the request context
func GetRole(r *http.Request) (string, bool) {
	role, ok := r.Context().Value(RoleKey).(string)
	return role, ok
}
