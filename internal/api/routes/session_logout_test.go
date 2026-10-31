package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestAuthSessionCookieFlow proves the routes-level session contract: the
// logout endpoint revokes the session and clears the cookie with matching
// attributes, anonymous logout is rejected by the auth chain, MFA-verify is
// CSRF-protected now that it sets a session, and the password step stays
// CSRF-exempt.
func TestAuthSessionCookieFlow(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		withAuth   bool
		crossSite  bool
		wantStatus int
		wantCode   string
	}{
		{
			name:       "error - anonymous logout is rejected by the auth chain",
			method:     http.MethodPost,
			path:       "/api/auth/logout",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MISSING_TOKEN",
		},
		{
			name:       "error - cross-site logout is rejected by CSRF first",
			method:     http.MethodPost,
			path:       "/api/auth/logout",
			withAuth:   true,
			crossSite:  true,
			wantStatus: http.StatusForbidden,
			wantCode:   "CSRF_VALIDATION_FAILED",
		},
		{
			name:       "error - cross-site MFA-verify is rejected by CSRF",
			method:     http.MethodPost,
			path:       "/api/auth/mfa/verify",
			crossSite:  true,
			wantStatus: http.StatusForbidden,
			wantCode:   "CSRF_VALIDATION_FAILED",
		},
		{
			name:       "success - password step stays CSRF-exempt",
			method:     http.MethodPost,
			path:       "/api/auth/login",
			crossSite:  true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "error - logout rejects a wrong method",
			method:     http.MethodGet,
			path:       "/api/auth/logout",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, deps := setupTestRouterWithBearerDeps(t)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.withAuth {
				token, err := deps.jwtManager.GenerateToken("1", "user", "Admin", 0)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}
			if tt.crossSite {
				req.Header.Set("Origin", "https://evil.com")
				req.Header.Set("Sec-Fetch-Site", "cross-site")
				req.Header.Set("Sec-Fetch-Mode", "cors")
			}

			code, envelope := envelopeCode(t, r, req)

			assert.Equal(t, tt.wantStatus, code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelope)
			}
		})
	}
}

// TestAuthLogoutRoute_RevokesAndClears proves an authenticated logout revokes
// the session server-side and clears the session cookie with matching
// attributes.
func TestAuthLogoutRoute_RevokesAndClears(t *testing.T) {
	r, deps := setupTestRouterWithBearerDeps(t)

	deps.userRepo.EXPECT().RevokeSessions(mock.Anything, 1).Return(nil)

	token, err := deps.jwtManager.GenerateToken("1", "user", "Admin", 0)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// HTTPS deployment: the cleared cookie must carry Secure to match.
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1, "one cleared cookie")
	cookie := cookies[0]
	assert.Equal(t, "lesstruct_jwt", cookie.Name, "cleared cookie name")
	assert.Equal(t, "/", cookie.Path, "cleared cookie path")
	assert.True(t, cookie.HttpOnly, "cleared cookie is HttpOnly")
	assert.True(t, cookie.Secure, "cleared cookie is Secure on HTTPS")
	assert.Equal(t, -1, cookie.MaxAge, "cleared cookie expires immediately")
}
