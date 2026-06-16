package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestNoCookieMiddleware(t *testing.T) {
	logger := util.NewLogger(os.Stdout)

	tests := []struct {
		name            string
		handler         http.HandlerFunc
		expectCookies   bool
		expectedBody    string
		expectedStatus  int
	}{
		{
			name: "strips Set-Cookie header from response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123"})
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			},
			expectCookies:  false,
			expectedBody:   "ok",
			expectedStatus: http.StatusOK,
		},
		{
			name: "strips multiple Set-Cookie headers",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123"})
				http.SetCookie(w, &http.Cookie{Name: "tracking", Value: "xyz789"})
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			},
			expectCookies:  false,
			expectedBody:   "ok",
			expectedStatus: http.StatusOK,
		},
		{
			name: "does not interfere with responses without cookies",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			},
			expectCookies:  false,
			expectedBody:   `{"status":"ok"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name: "preserves other headers when stripping cookies",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Custom-Header", "preserved")
				http.SetCookie(w, &http.Cookie{Name: "bad", Value: "cookie"})
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("created"))
			},
			expectCookies:  false,
			expectedBody:   "created",
			expectedStatus: http.StatusCreated,
		},
		{
			name: "strips cookies when Write is called without WriteHeader",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.SetCookie(w, &http.Cookie{Name: "implicit", Value: "test"})
				_, _ = w.Write([]byte("implicit 200"))
			},
			expectCookies:  false,
			expectedBody:   "implicit 200",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := middleware.NewNoCookieMiddleware(logger)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			rec := httptest.NewRecorder()

			handler := m.Handler(http.HandlerFunc(tt.handler))
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code, "status code")
			assert.Equal(t, tt.expectedBody, rec.Body.String(), "response body")

			cookies := rec.Header().Values("Set-Cookie")
			if tt.expectCookies {
				assert.NotEmpty(t, cookies, "expected Set-Cookie headers to be present")
			} else {
				assert.Empty(t, cookies, "expected Set-Cookie headers to be stripped")
			}
		})
	}
}

func TestNoCookieMiddleware_PreservesCustomHeaders(t *testing.T) {
	logger := util.NewLogger(os.Stdout)
	m := middleware.NewNoCookieMiddleware(logger)

	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "test-123")
		w.Header().Set("Content-Type", "application/json")
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "leaked"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"value"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "test-123", rec.Header().Get("X-Request-Id"), "X-Request-Id should be preserved")
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"), "Content-Type should be preserved")
	assert.Empty(t, rec.Header().Values("Set-Cookie"), "Set-Cookie should be stripped")
}

func TestNoCookieMiddleware_WorksAcrossMethods(t *testing.T) {
	logger := util.NewLogger(os.Stdout)
	m := middleware.NewNoCookieMiddleware(logger)

	cookieSettingHandler := func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "leak", Value: "data"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/test", nil)
			rec := httptest.NewRecorder()

			handler := m.Handler(http.HandlerFunc(cookieSettingHandler))
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "status code for %s", method)
			assert.Empty(t, rec.Header().Values("Set-Cookie"), "Set-Cookie stripped for %s", method)
		})
	}
}

func TestNoCookieMiddleware_LoginReturnsTokenInBody(t *testing.T) {
	logger := util.NewLogger(os.Stdout)
	m := middleware.NewNoCookieMiddleware(logger)

	loginHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"jwt-token-here","refresh_token":"refresh-here"}`))
	})

	handler := m.Handler(loginHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"), "login must not set cookies")
	assert.Contains(t, rec.Body.String(), "access_token", "token must be in response body")
	assert.Contains(t, rec.Body.String(), "refresh_token", "refresh token must be in response body")
}

func TestNoCookieMiddleware_BearerAuthPassesThrough(t *testing.T) {
	logger := util.NewLogger(os.Stdout)
	m := middleware.NewNoCookieMiddleware(logger)

	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"auth":"` + authHeader + `"}`))
	})

	handler := m.Handler(protectedHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items", nil)
	req.Header.Set("Authorization", "Bearer test-jwt-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"), "protected endpoint must not set cookies")
	assert.Contains(t, rec.Body.String(), "Bearer test-jwt-token", "Authorization header must pass through")
}
