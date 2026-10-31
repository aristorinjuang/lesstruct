package middleware

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	apikeydomain "github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/kernel"
)

const (
	UserIDKey   contextKey = "user_id"
	UsernameKey contextKey = "username"
	RoleKey     contextKey = "role"
)

// Session validation constants
const (
	// accountStatusVerified is the only account status allowed to carry a
	// session. Suspended and soft-deleted accounts are rejected on the next
	// request instead of at token expiry.
	accountStatusVerified = "verified"
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

// isAPIKeyShaped reports whether a Bearer token looks like a lesstruct API
// key (the "lesstruct_" prefix). API keys belong to the Bearer-only /api/v1
// agent realm; JWT routes must reject them with JWT_REQUIRED guidance instead
// of the generic INVALID_TOKEN.
func isAPIKeyShaped(token string) bool {
	return strings.HasPrefix(token, apikeydomain.KeyPrefix)
}

// newUserContext populates request context with user identity using the shared
// context keys. Both the JWT AuthMiddleware and the API-key APIKeyAuthMiddleware
// route through this so downstream ctxUser lookups and RBAC are auth-agnostic
// (architecture Important Gap #1). Values are strings (UserID is the decimal id).
// A kernel.Principal is attached alongside for the v1 identity seam; the string
// keys stay until Phase 3 removes them per module.
func newUserContext(ctx context.Context, userID, username, role string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, UsernameKey, username)
	ctx = context.WithValue(ctx, RoleKey, role)
	roles := make([]string, 0, 1)
	if role != "" {
		roles = append(roles, role)
	}
	return kernel.ContextWithPrincipal(ctx, kernel.Principal{
		ID:       userID,
		Username: username,
		Roles:    roles,
	})
}

// clearStaleSessionCookie drops a presented-but-rejected session cookie so an
// expired or revoked session stops being resent on every request. It only
// acts when no Authorization header was sent: a present header means the
// caller authenticated with a Bearer and any cookie was never consulted.
func clearStaleSessionCookie(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "" {
		return
	}
	if _, err := r.Cookie(kernel.SessionCookieName); err != nil {
		return
	}
	kernel.ClearSessionCookie(w, r)
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
	userLookup UserLookup
}

// extractToken extracts the JWT from the Authorization header with a cookie
// fallback. A present header always wins — even an API-key-shaped Bearer is
// returned as-is so the caller rejects it with JWT_REQUIRED without ever
// consulting the cookie. The session cookie is only read when no header is
// present, so already-issued Bearer JWTs keep validating exactly as today.
func (m *AuthMiddleware) extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
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

	if cookie, err := r.Cookie(kernel.SessionCookieName); err == nil {
		token := strings.TrimSpace(cookie.Value)
		if token != "" {
			return token, nil
		}
	}

	return "", ErrMissingToken
}

// createUserContext now delegates to the shared helper.
func (m *AuthMiddleware) createUserContext(ctx context.Context, claims *auth.Claims) context.Context {
	return newUserContext(ctx, claims.UserID, claims.Username, claims.Role)
}

// hasRequiredRole checks if the user role is in the list of required roles
func (m *AuthMiddleware) hasRequiredRole(userRole string, requiredRoles []string) bool {
	return slices.Contains(requiredRoles, userRole)
}

// sessionValid resolves the token subject and verifies the session is still
// usable: the account exists, is verified, and its stored token version still
// matches the one embedded in the token. A password change or reset bumps the
// stored version, so every token issued before it is rejected immediately
// instead of remaining valid until expiry. Every failure reports false without
// revealing which check failed.
func (m *AuthMiddleware) sessionValid(ctx context.Context, claims *auth.Claims) bool {
	userID, err := strconv.Atoi(claims.UserID)
	if err != nil {
		return false
	}

	user, err := m.userLookup.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return false
	}

	if user.Status != accountStatusVerified {
		return false
	}

	return claims.TokenVersion == user.TokenVersion
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
		if isAPIKeyShaped(token) {
			response.Error(w, http.StatusUnauthorized, "JWT_REQUIRED", "This endpoint requires a JWT; API keys belong on the /api/v1 agent endpoints", nil)
			return
		}

		// Validate token
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			// Check if token is expired
			if strings.Contains(err.Error(), "expired") {
				clearStaleSessionCookie(w, r)
				response.Error(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Your session has expired. Please log in again", nil)
				return
			}

			// Invalid token
			clearStaleSessionCookie(w, r)
			response.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid authorization token", nil)
			return
		}

		// Reject revoked sessions (password change/reset) and accounts that are
		// no longer active before the request reaches a handler.
		if !m.sessionValid(r.Context(), claims) {
			clearStaleSessionCookie(w, r)
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

		// A revoked or inactive session is treated as anonymous; the handler
		// decides what an anonymous caller may see.
		if !m.sessionValid(r.Context(), claims) {
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
			if isAPIKeyShaped(token) {
				response.Error(w, http.StatusUnauthorized, "JWT_REQUIRED", "This endpoint requires a JWT; API keys belong on the /api/v1 agent endpoints", nil)
				return
			}

			// Validate token
			claims, err := m.jwtManager.ValidateToken(token)
			if err != nil {
				// Check if token is expired
				if strings.Contains(err.Error(), "expired") {
					clearStaleSessionCookie(w, r)
					response.Error(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Your session has expired. Please log in again", nil)
					return
				}

				// Invalid token
				clearStaleSessionCookie(w, r)
				response.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid authorization token", nil)
				return
			}

			// Reject revoked or inactive sessions before checking the role so a
			// stale token never reaches a protected handler.
			if !m.sessionValid(r.Context(), claims) {
				clearStaleSessionCookie(w, r)
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

// NewAuthMiddleware creates a new authentication middleware. userLookup is
// consulted on every authenticated request to enforce account status and
// token-version revocation, so a password change or deleted account takes
// effect immediately rather than at token expiry.
func NewAuthMiddleware(jwtManager *auth.JWTManager, userLookup UserLookup) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
		userLookup: userLookup,
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

// Principal retrieves the kernel identity from the request context.
// It is populated by both auth realms via newUserContext; ok is false on
// anonymous requests where no auth middleware ran.
func Principal(r *http.Request) (kernel.Principal, bool) {
	return kernel.PrincipalFromContext(r.Context())
}
