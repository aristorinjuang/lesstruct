package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expiredSessionToken mints a correctly signed but already-expired JWT so the
// cookie path proves it reports TOKEN_EXPIRED exactly like a bad Bearer.
func expiredSessionToken(t *testing.T, secret, userID string) string {
	t.Helper()

	claims := appauth.Claims{
		UserID:       userID,
		Username:     "testuser",
		Role:         "Contributor",
		TokenVersion: 0,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err, "Failed to sign expired token")
	return signed
}

// TestAuthMiddleware_RequireAuth_CookieFallback proves the session cookie is
// accepted exactly like the Bearer it replaces: cookie-only succeeds,
// a present header wins, an API-key header never falls through to the
// cookie, and a stale cookie reports the same codes as a stale Bearer.
func TestAuthMiddleware_RequireAuth_CookieFallback(t *testing.T) {
	jwtManager := appauth.NewJWTManager(sessionTestSecret)
	headerToken, err := jwtManager.GenerateToken("123", "testuser", "Contributor", 0)
	require.NoError(t, err, "Failed to generate header token")
	cookieToken, err := jwtManager.GenerateToken("456", "otheruser", "Contributor", 0)
	require.NoError(t, err, "Failed to generate cookie token")

	tests := []struct {
		name       string
		header     string
		cookie     string
		lookup     stubUserLookup
		wantStatus int
		wantCode   string
		wantUserID string
	}{
		{
			name:       "success - cookie-only session is accepted",
			cookie:     cookieToken,
			lookup:     stubUserLookup{},
			wantStatus: http.StatusOK,
			wantUserID: "456",
		},
		{
			name:       "success - header wins over the cookie",
			header:     "Bearer " + headerToken,
			cookie:     cookieToken,
			lookup:     stubUserLookup{},
			wantStatus: http.StatusOK,
			wantUserID: "123",
		},
		{
			name:       "error - API-key header never consults the cookie",
			header:     "Bearer lesstruct_abc123def456_secretsecretsecretsecret12",
			cookie:     cookieToken,
			lookup:     stubUserLookup{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "JWT_REQUIRED",
		},
		{
			name:       "error - revoked cookie matches the revoked Bearer code",
			cookie:     cookieToken,
			lookup:     stubUserLookup{user: activeSessionUser(9)},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			name:       "error - expired cookie matches the expired Bearer code",
			cookie:     expiredSessionToken(t, sessionTestSecret, "123"),
			lookup:     stubUserLookup{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "TOKEN_EXPIRED",
		},
		{
			name:       "error - forged cookie matches the forged Bearer code",
			cookie:     "forged-cookie-token",
			lookup:     stubUserLookup{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager, tt.lookup)

			var gotUserID string
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUserID, _ = appmiddleware.GetUserID(r)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: kernel.SessionCookieName, Value: tt.cookie})
			}

			rr := httptest.NewRecorder()
			authMiddleware.RequireAuth(nextHandler).ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantCode != "" {
				assert.Contains(t, rr.Body.String(), tt.wantCode)
				assert.Empty(t, gotUserID, "a rejected session must not reach the handler")
				return
			}
			assert.Equal(t, tt.wantUserID, gotUserID)
		})
	}
}

// TestAuthMiddleware_CookieFallback_Guards proves the other two guards honor
// the cookie: RequireRole attaches the principal for role checks and
// OptionalAuth treats a valid cookie as a session and a revoked one as
// anonymous.
func TestAuthMiddleware_CookieFallback_Guards(t *testing.T) {
	jwtManager := appauth.NewJWTManager(sessionTestSecret)
	adminToken, err := jwtManager.GenerateToken("7", "alice", "Admin", 0)
	require.NoError(t, err, "Failed to generate admin token")
	revokedToken, err := jwtManager.GenerateToken("7", "alice", "Admin", 0)
	require.NoError(t, err, "Failed to generate revoked token")

	tests := []struct {
		name        string
		guard       string
		cookie      string
		lookup      stubUserLookup
		wantStatus  int
		wantPresent bool
	}{
		{
			name:        "success - RequireRole accepts a cookie session",
			guard:       "role",
			cookie:      adminToken,
			lookup:      stubUserLookup{user: activeSessionUser(0)},
			wantStatus:  http.StatusOK,
			wantPresent: true,
		},
		{
			name:        "success - OptionalAuth attaches a cookie session",
			guard:       "optional",
			cookie:      adminToken,
			lookup:      stubUserLookup{user: activeSessionUser(0)},
			wantStatus:  http.StatusOK,
			wantPresent: true,
		},
		{
			name:        "success - OptionalAuth treats a revoked cookie as anonymous",
			guard:       "optional",
			cookie:      revokedToken,
			lookup:      stubUserLookup{user: activeSessionUser(3)},
			wantStatus:  http.StatusOK,
			wantPresent: false,
		},
		{
			name:        "success - OptionalAuth without credential stays anonymous",
			guard:       "optional",
			lookup:      stubUserLookup{},
			wantStatus:  http.StatusOK,
			wantPresent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager, tt.lookup)

			var present bool
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, present = appmiddleware.GetUserID(r)
				w.WriteHeader(http.StatusOK)
			})

			var handler http.Handler
			switch tt.guard {
			case "role":
				handler = authMiddleware.RequireRole("Admin")(nextHandler)
			default:
				handler = authMiddleware.OptionalAuth(nextHandler)
			}

			req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: kernel.SessionCookieName, Value: tt.cookie})
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, tt.wantPresent, present)
		})
	}
}

// TestAuthMiddleware_ClearsStaleSessionCookie proves a rejected cookie session
// is cleared on the way out so expired or revoked sessions stop being resent,
// while Bearer-authenticated requests never touch the cookie jar.
func TestAuthMiddleware_ClearsStaleSessionCookie(t *testing.T) {
	jwtManager := appauth.NewJWTManager(sessionTestSecret)
	revokedToken, err := jwtManager.GenerateToken("123", "testuser", "Contributor", 0)
	require.NoError(t, err, "Failed to generate revoked token")

	tests := []struct {
		name        string
		header      string
		cookie      string
		lookup      stubUserLookup
		wantCleared bool
	}{
		{
			name:        "cleared - expired cookie without header",
			cookie:      expiredSessionToken(t, sessionTestSecret, "123"),
			lookup:      stubUserLookup{},
			wantCleared: true,
		},
		{
			name:        "cleared - revoked cookie without header",
			cookie:      revokedToken,
			lookup:      stubUserLookup{user: activeSessionUser(9)},
			wantCleared: true,
		},
		{
			name:        "cleared - forged cookie without header",
			cookie:      "forged-cookie-token",
			lookup:      stubUserLookup{},
			wantCleared: true,
		},
		{
			name:        "kept - no cookie means nothing to clear",
			lookup:      stubUserLookup{},
			wantCleared: false,
		},
		{
			name:        "kept - Bearer rejection never touches the cookie jar",
			header:      "Bearer forged-bearer-token",
			cookie:      revokedToken,
			lookup:      stubUserLookup{user: activeSessionUser(9)},
			wantCleared: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager, tt.lookup)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: kernel.SessionCookieName, Value: tt.cookie})
			}

			rr := httptest.NewRecorder()
			authMiddleware.RequireAuth(nextHandler).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			cleared := false
			for _, c := range rr.Result().Cookies() {
				if c.Name == kernel.SessionCookieName && c.MaxAge < 0 {
					cleared = true
				}
			}
			assert.Equal(t, tt.wantCleared, cleared, "stale cookie clearing")
		})
	}
}
