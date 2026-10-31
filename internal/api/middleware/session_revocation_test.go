package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sessionTestSecret = "test-secret-key-for-testing-purposes-min-32-chars"

func activeSessionUser(tokenVersion int) *userdomain.User {
	return &userdomain.User{
		ID:           123,
		Username:     "testuser",
		Role:         "Contributor",
		Status:       "verified",
		TokenVersion: tokenVersion,
	}
}

// TestAuthMiddleware_RequireAuth_SessionValidation proves a signed token is not
// enough on its own: the account must still exist, be active, and carry the
// same token version. A password change bumps the version, and a deleted or
// suspended account is rejected on the next request instead of at expiry.
func TestAuthMiddleware_RequireAuth_SessionValidation(t *testing.T) {
	jwtManager := appauth.NewJWTManager(sessionTestSecret)

	tests := []struct {
		name         string
		userID       string
		tokenVersion int
		lookup       stubUserLookup
		wantStatus   int
		wantCode     string
	}{
		{
			name:         "success - active account with matching version",
			userID:       "123",
			tokenVersion: 4,
			lookup:       stubUserLookup{user: activeSessionUser(4)},
			wantStatus:   http.StatusOK,
		},
		{
			name:         "error - password change bumped the stored version",
			userID:       "123",
			tokenVersion: 4,
			lookup:       stubUserLookup{user: activeSessionUser(5)},
			wantStatus:   http.StatusUnauthorized,
			wantCode:     "INVALID_TOKEN",
		},
		{
			name:         "error - soft-deleted account",
			userID:       "123",
			tokenVersion: 0,
			lookup: stubUserLookup{user: &userdomain.User{
				ID: 123, Username: "testuser", Role: "Contributor", Status: "soft_deleted",
			}},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			name:         "error - suspended account",
			userID:       "123",
			tokenVersion: 0,
			lookup: stubUserLookup{user: &userdomain.User{
				ID: 123, Username: "testuser", Role: "Contributor", Status: "suspended",
			}},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			name:         "error - account no longer exists",
			userID:       "123",
			tokenVersion: 0,
			lookup:       stubUserLookup{missing: true},
			wantStatus:   http.StatusUnauthorized,
			wantCode:     "INVALID_TOKEN",
		},
		{
			name:         "error - account lookup fails",
			userID:       "123",
			tokenVersion: 0,
			lookup:       stubUserLookup{err: errors.New("database unavailable")},
			wantStatus:   http.StatusUnauthorized,
			wantCode:     "INVALID_TOKEN",
		},
		{
			name:         "error - user id claim is not numeric",
			userID:       "not-a-number",
			tokenVersion: 0,
			lookup:       stubUserLookup{user: activeSessionUser(0)},
			wantStatus:   http.StatusUnauthorized,
			wantCode:     "INVALID_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager, tt.lookup)
			token, err := jwtManager.GenerateToken(tt.userID, "testuser", "Contributor", tt.tokenVersion)
			require.NoError(t, err)

			var gotUserID string
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUserID, _ = appmiddleware.GetUserID(r)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rr := httptest.NewRecorder()
			authMiddleware.RequireAuth(nextHandler).ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantCode != "" {
				assert.Contains(t, rr.Body.String(), tt.wantCode)
				assert.Empty(t, gotUserID, "a rejected session must not reach the handler")
				return
			}
			assert.Equal(t, tt.userID, gotUserID)
		})
	}
}

// TestAuthMiddleware_OptionalAuth_RevokedSessionIsAnonymous proves a revoked
// token on an optional-auth route is treated as no session at all, so the
// handler serves the public view instead of the account's view.
func TestAuthMiddleware_OptionalAuth_RevokedSessionIsAnonymous(t *testing.T) {
	jwtManager := appauth.NewJWTManager(sessionTestSecret)

	tests := []struct {
		name        string
		lookup      stubUserLookup
		wantPresent bool
	}{
		{
			name:        "success - active session is attached",
			lookup:      stubUserLookup{user: activeSessionUser(2)},
			wantPresent: true,
		},
		{
			name:        "success - revoked session is anonymous",
			lookup:      stubUserLookup{user: activeSessionUser(3)},
			wantPresent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager, tt.lookup)
			token, err := jwtManager.GenerateToken("123", "testuser", "Contributor", 2)
			require.NoError(t, err)

			var present bool
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, present = appmiddleware.GetUserID(r)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/public", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rr := httptest.NewRecorder()
			authMiddleware.OptionalAuth(nextHandler).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.wantPresent, present)
		})
	}
}

// TestAuthMiddleware_RequireRole_RevokedSessionIsRejected proves role gates run
// only for a live session: a revoked admin token gets 401, not 403 or 200.
func TestAuthMiddleware_RequireRole_RevokedSessionIsRejected(t *testing.T) {
	jwtManager := appauth.NewJWTManager(sessionTestSecret)
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{user: activeSessionUser(8)})

	token, err := jwtManager.GenerateToken("123", "admin", "Admin", 7)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	authMiddleware.RequireRole("Admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_TOKEN")
}
