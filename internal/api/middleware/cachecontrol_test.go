package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheControlMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantHeader string
		wantBody   string
		wantStatus int
	}{
		{
			name:       "stamps the immutable value",
			value:      "public, max-age=31536000, immutable",
			wantHeader: "public, max-age=31536000, immutable",
			wantBody:   "ok",
			wantStatus: http.StatusOK,
		},
		{
			name:       "stamps the bounded value",
			value:      "public, max-age=86400",
			wantHeader: "public, max-age=86400",
			wantBody:   "ok",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/asset", nil)
			middleware.NewCacheControlMiddleware(tt.value).Handler(next).ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
			assert.Equal(t, tt.wantHeader, w.Header().Get("Cache-Control"))
		})
	}
}
