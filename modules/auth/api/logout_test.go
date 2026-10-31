package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/auth/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logoutHandler builds a handler carrying only the user store the Logout
// endpoint touches.
func logoutHandler(store api.UserStore) *api.AuthHandler {
	return api.NewAuthHandler(
		nil,
		&stubIssuer{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		store,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

// withPrincipal attaches the kernel identity a session middleware would have
// injected.
func withPrincipal(req *http.Request, id, username string) *http.Request {
	return req.WithContext(kernel.ContextWithPrincipal(req.Context(), kernel.Principal{
		ID:       id,
		Username: username,
		Roles:    []string{"Admin"},
	}))
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name        string
		principalID string
		username    string
		anonymous   bool
		store       *stubUserStore
		wantStatus  int
		wantCode    string
		wantRevoked []int
	}{
		{
			name:        "success - revokes sessions and clears the cookie",
			principalID: "1",
			username:    "admin",
			store:       &stubUserStore{},
			wantStatus:  http.StatusOK,
			wantRevoked: []int{1},
		},
		{
			name:       "error - anonymous request is rejected",
			anonymous:  true,
			store:      &stubUserStore{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "MISSING_TOKEN",
		},
		{
			name:        "error - non-numeric subject is rejected",
			principalID: "not-a-number",
			username:    "admin",
			store:       &stubUserStore{},
			wantStatus:  http.StatusUnauthorized,
			wantCode:    "INVALID_TOKEN",
		},
		{
			name:        "error - revocation failure",
			principalID: "1",
			username:    "admin",
			store:       &stubUserStore{updateErr: errors.New("database error")},
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "LOGOUT_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := logoutHandler(tt.store)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			if !tt.anonymous {
				req = withPrincipal(req, tt.principalID, tt.username)
			}
			w := httptest.NewRecorder()

			handler.Logout(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "status")

			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, decodeErrorCode(t, w), "error code")
				assert.Empty(t, tt.store.revoked, "a rejected logout must not revoke")
				return
			}

			assert.Equal(t, tt.wantRevoked, tt.store.revoked, "revoked sessions")

			cookies := w.Result().Cookies()
			require.Len(t, cookies, 2, "session + hint cleared")
			var session, hint *http.Cookie
			for _, c := range cookies {
				switch c.Name {
				case "lesstruct_jwt":
					session = c
				case "lesstruct_auth":
					hint = c
				}
			}
			require.NotNil(t, session, "session cookie present")
			require.NotNil(t, hint, "auth hint cookie present")
			assert.Equal(t, "lesstruct_jwt", session.Name, "cleared cookie name")
			assert.Equal(t, "/", session.Path, "cleared cookie path")
			assert.True(t, session.HttpOnly, "cleared cookie is HttpOnly")
			assert.Equal(t, -1, session.MaxAge, "cleared cookie expires immediately")
			assert.Equal(t, "lesstruct_auth", hint.Name, "cleared hint name")
			assert.Equal(t, "/", hint.Path, "cleared hint path")
			assert.False(t, hint.HttpOnly, "cleared hint is readable")
			assert.Equal(t, -1, hint.MaxAge, "cleared hint expires immediately")

			data := decodeData(t, w)
			assert.Equal(t, "Logged out successfully.", data["message"], "message")
		})
	}
}

func TestLogout_RevocationBumpInvalidatesBearer(t *testing.T) {
	store := &stubUserStore{}
	handler := logoutHandler(store)

	req := withPrincipal(httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil), "7", "alice")
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	require.Equal(t, http.StatusOK, w.Code, "status")
	assert.Equal(t, []int{7}, store.revoked, "the subject's sessions are revoked")
}

func TestLogout_ClearsSecureCookieOnHTTPS(t *testing.T) {
	tests := []struct {
		name           string
		forwardedProto string
		wantSecure     bool
	}{
		{
			name:       "plain HTTP clears without Secure",
			wantSecure: false,
		},
		{
			name:           "forwarded https clears with Secure",
			forwardedProto: "https",
			wantSecure:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubUserStore{}
			handler := logoutHandler(store)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			req = withPrincipal(req, "1", "admin")
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			w := httptest.NewRecorder()

			handler.Logout(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			cookies := w.Result().Cookies()
			require.Len(t, cookies, 2, "session + hint cleared")
			var session, hint *http.Cookie
			for _, c := range cookies {
				switch c.Name {
				case "lesstruct_jwt":
					session = c
				case "lesstruct_auth":
					hint = c
				}
			}
			require.NotNil(t, session, "session cookie present")
			require.NotNil(t, hint, "auth hint cookie present")
			assert.Equal(t, tt.wantSecure, session.Secure, "cleared cookie Secure flag")
			assert.Equal(t, tt.wantSecure, hint.Secure, "cleared hint Secure flag")
		})
	}
}
