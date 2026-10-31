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
			require.Len(t, cookies, 1, "one cleared cookie")
			cookie := cookies[0]
			assert.Equal(t, "lesstruct_jwt", cookie.Name, "cleared cookie name")
			assert.Equal(t, "/", cookie.Path, "cleared cookie path")
			assert.True(t, cookie.HttpOnly, "cleared cookie is HttpOnly")
			assert.Equal(t, -1, cookie.MaxAge, "cleared cookie expires immediately")

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
			require.Len(t, cookies, 1, "one cleared cookie")
			assert.Equal(t, tt.wantSecure, cookies[0].Secure, "cleared cookie Secure flag")
		})
	}
}
