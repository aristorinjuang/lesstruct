package kernel_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCookieName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "success - canonical session cookie name",
			expected: "lesstruct_jwt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, kernel.SessionCookieName)
		})
	}
}

func TestAuthHintCookieName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "success - canonical auth hint cookie name",
			expected: "lesstruct_auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, kernel.AuthHintCookieName)
		})
	}
}

func TestRequestIsHTTPS(t *testing.T) {
	tests := []struct {
		name           string
		tls            bool
		forwardedProto string
		wantHTTPS      bool
	}{
		{
			name:      "success - plain HTTP is not HTTPS",
			wantHTTPS: false,
		},
		{
			name:      "success - TLS is HTTPS",
			tls:       true,
			wantHTTPS: true,
		},
		{
			name:           "success - forwarded https behind a terminating proxy is HTTPS",
			forwardedProto: "https",
			wantHTTPS:      true,
		},
		{
			name:           "success - forwarded http is not HTTPS",
			tls:            true,
			forwardedProto: "http",
			wantHTTPS:      false,
		},
		{
			name:           "success - first forwarded value wins",
			forwardedProto: "https, http",
			wantHTTPS:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			assert.Equal(t, tt.wantHTTPS, kernel.RequestIsHTTPS(req))
		})
	}
}

func TestSetSessionCookie(t *testing.T) {
	tests := []struct {
		name        string
		tls         bool
		wantSecure  bool
		wantExpired bool
	}{
		{
			name:        "success - plain HTTP omits Secure",
			wantSecure:  false,
			wantExpired: false,
		},
		{
			name:        "success - TLS sets Secure",
			tls:         true,
			wantSecure:  true,
			wantExpired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()
			expires := time.Now().Add(time.Hour)
			kernel.SetSessionCookie(w, req, "session-jwt", expires)

			cookies := w.Result().Cookies()
			require.Len(t, cookies, 2)
			var session, hint *http.Cookie
			for _, c := range cookies {
				switch c.Name {
				case kernel.SessionCookieName:
					session = c
				case kernel.AuthHintCookieName:
					hint = c
				}
			}
			require.NotNil(t, session, "session cookie present")
			require.NotNil(t, hint, "auth hint cookie present")
			assert.Equal(t, kernel.SessionCookieName, session.Name)
			assert.Equal(t, "session-jwt", session.Value)
			assert.Equal(t, "/", session.Path)
			assert.True(t, session.HttpOnly)
			assert.Equal(t, tt.wantSecure, session.Secure)
			assert.Equal(t, http.SameSiteLaxMode, session.SameSite)
			assert.WithinDuration(t, expires, session.Expires, time.Minute)
			assert.Equal(t, kernel.AuthHintCookieName, hint.Name)
			assert.Equal(t, "1", hint.Value)
			assert.Equal(t, "/", hint.Path)
			assert.False(t, hint.HttpOnly)
			assert.Equal(t, tt.wantSecure, hint.Secure)
			assert.Equal(t, http.SameSiteLaxMode, hint.SameSite)
			assert.WithinDuration(t, expires, hint.Expires, time.Minute)
			assert.Equal(t, session.MaxAge, hint.MaxAge)
		})
	}
}

func TestClearSessionCookie(t *testing.T) {
	tests := []struct {
		name           string
		tls            bool
		forwardedProto string
		wantSecure     bool
	}{
		{
			name:       "success - plain HTTP clears without Secure",
			wantSecure: false,
		},
		{
			name:       "success - TLS clears with Secure",
			tls:        true,
			wantSecure: true,
		},
		{
			name:           "success - forwarded https clears with Secure",
			forwardedProto: "https",
			wantSecure:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			w := httptest.NewRecorder()
			kernel.ClearSessionCookie(w, req)

			cookies := w.Result().Cookies()
			require.Len(t, cookies, 2)
			var session, hint *http.Cookie
			for _, c := range cookies {
				switch c.Name {
				case kernel.SessionCookieName:
					session = c
				case kernel.AuthHintCookieName:
					hint = c
				}
			}
			require.NotNil(t, session, "session cookie present")
			require.NotNil(t, hint, "auth hint cookie present")
			assert.Equal(t, kernel.SessionCookieName, session.Name)
			assert.Equal(t, "/", session.Path)
			assert.True(t, session.HttpOnly)
			assert.Equal(t, tt.wantSecure, session.Secure)
			assert.Equal(t, http.SameSiteLaxMode, session.SameSite)
			assert.Equal(t, -1, session.MaxAge)
			assert.Equal(t, kernel.AuthHintCookieName, hint.Name)
			assert.Equal(t, "/", hint.Path)
			assert.False(t, hint.HttpOnly)
			assert.Equal(t, tt.wantSecure, hint.Secure)
			assert.Equal(t, http.SameSiteLaxMode, hint.SameSite)
			assert.Equal(t, -1, hint.MaxAge)
		})
	}
}
