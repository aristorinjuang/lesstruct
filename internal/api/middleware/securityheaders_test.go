package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersMiddleware_SetsAllHeaders(t *testing.T) {
	mw := middleware.NewSecurityHeadersMiddleware("Content-Security-Policy", "default-src 'self'", "DENY")

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeadersMiddleware_EmptyCSPValue(t *testing.T) {
	mw := middleware.NewSecurityHeadersMiddleware("Content-Security-Policy", "", "DENY")

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Static security headers still set
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))

	// CSP omitted when value is empty
	assert.Empty(t, w.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeadersMiddleware_ReportOnlyHeaderName(t *testing.T) {
	mw := middleware.NewSecurityHeadersMiddleware("Content-Security-Policy-Report-Only", "default-src 'self'", "DENY")

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy-Report-Only"))
	assert.Empty(t, w.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeadersMiddleware_XFrameOptions(t *testing.T) {
	tests := []struct {
		name          string
		xFrameOptions string
		want          string
	}{
		{
			name:          "deny default",
			xFrameOptions: "DENY",
			want:          "DENY",
		},
		{
			name:          "same origin",
			xFrameOptions: "SAMEORIGIN",
			want:          "SAMEORIGIN",
		},
		{
			name:          "omitted for host allowlists",
			xFrameOptions: "",
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := middleware.NewSecurityHeadersMiddleware("Content-Security-Policy", "default-src 'self'", tt.xFrameOptions)

			handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.want, w.Header().Get("X-Frame-Options"))
		})
	}
}
