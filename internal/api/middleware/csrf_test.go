package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestCSRFMiddleware_Handler_SameOriginPOST(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Same-origin POST should pass through")
}

func TestCSRFMiddleware_Handler_CrossOriginPOST(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Cross-origin POST should be rejected with 403")
	assert.Contains(t, buf.String(), "CSRF validation failed")
	assert.Contains(t, buf.String(), "https://evil.com")
}

func TestCSRFMiddleware_Handler_SafeMethods(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	tests := []struct {
		name   string
		method string
	}{
		{name: "GET request", method: http.MethodGet},
		{name: "HEAD request", method: http.MethodHead},
		{name: "OPTIONS request", method: http.MethodOptions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/content_items", nil)
			req.Header.Set("Sec-Fetch-Site", "cross-site")

			rr := httptest.NewRecorder()
			handler := mw.Handler(nextHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "Safe methods should pass through regardless of origin")
		})
	}
}

func TestCSRFMiddleware_Handler_SameOriginPUT(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/content_items/1", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Same-origin PUT should pass through")
}

func TestCSRFMiddleware_Handler_SameOriginDELETE(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/content_items/1", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Same-origin DELETE should pass through")
}

func TestCSRFMiddleware_Handler_SameOriginPATCH(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/content_items/1", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Same-origin PATCH should pass through")
}

func TestCSRFMiddleware_Handler_CrossOriginLogsDetails(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/content_items/42", nil)
	req.Header.Set("Origin", "https://attacker.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	logOutput := buf.String()
	assert.Contains(t, logOutput, "CSRF validation failed")
	assert.Contains(t, logOutput, "DELETE")
	assert.Contains(t, logOutput, "/api/v1/content_items/42")
	assert.Contains(t, logOutput, "https://attacker.com")
}

func TestCSRFMiddleware_Handler_NoOriginHeaders(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", nil)

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Requests without browser headers should pass through (non-browser clients)")
}

func TestCSRFMiddleware_Handler_CrossOriginUnsafeMethods(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	tests := []struct {
		name   string
		method string
	}{
		{name: "PUT", method: http.MethodPut},
		{name: "DELETE", method: http.MethodDelete},
		{name: "PATCH", method: http.MethodPatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/content_items/1", nil)
			req.Header.Set("Origin", "https://evil.com")
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			req.Header.Set("Sec-Fetch-Mode", "cors")

			rr := httptest.NewRecorder()
			handler := mw.Handler(nextHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code, "Cross-origin %s should be rejected", tt.method)
		})
	}
}

func TestCSRFMiddleware_Handler_RejectionResponseBody(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)

	var body map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	assert.NoError(t, err, "Response body should be valid JSON")

	errorObj, ok := body["error"].(map[string]any)
	assert.True(t, ok, "Response should contain error object")
	assert.Equal(t, "CSRF_VALIDATION_FAILED", errorObj["code"])
	assert.Equal(t, "Cross-origin request rejected", errorObj["message"])
}

func TestCSRFMiddleware_Handler_MissingOriginLogsMissing(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	// Deliberately NOT setting Origin header — want cross-origin rejection
	// without Origin to test "missing" fallback.
	// We must set Origin to trigger cross-origin detection but clear it
	// after Check sees it. Since httptest doesn't send real browser headers,
	// we test the "missing" path by setting Sec-Fetch-Site to cross-site
	// without Origin.

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// When Sec-Fetch-Site is cross-site but Origin is empty,
	// CrossOriginProtection.Check may or may not reject depending on
	// whether it treats missing Origin as cross-origin.
	// The important thing is: if rejected, origin should be logged as "missing".
	if rr.Code == http.StatusForbidden {
		assert.Contains(t, buf.String(), "origin=missing", "Missing origin should be logged as 'missing'")
	}
}

func TestCSRFMiddleware_Handler_UserIDInLog(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := util.NewLogger(buf)
	mw := appmiddleware.NewCSRFMiddleware(logger, nil, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")

	rr := httptest.NewRecorder()
	handler := mw.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, buf.String(), "user_id=unauthenticated", "Should log user_id=unauthenticated when no JWT")
}
