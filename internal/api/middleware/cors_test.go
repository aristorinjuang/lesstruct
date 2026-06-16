package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
)

// testLogger creates a discard logger for CORS middleware tests
func testLogger() *util.Logger {
	return util.NewLogger(io.Discard)
}

// TestCORSMiddleware_AllowedOrigin tests that allowed origins receive proper CORS headers
func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080", "https://example.com"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with allowed origin
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "http://localhost:8080", rr.Header().Get("Access-Control-Allow-Origin"), "Expected correct CORS origin header")
	assert.Equal(t, "true", rr.Header().Get("Access-Control-Allow-Credentials"), "Expected credentials header")
}

// TestCORSMiddleware_UnallowedOrigin tests that unauthorized origins are rejected
func TestCORSMiddleware_UnallowedOrigin(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with unallowed origin
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://evil.com")

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 (request still processed)")
	assert.Equal(t, "", rr.Header().Get("Access-Control-Allow-Origin"), "Expected no CORS origin header for unauthorized origin")
	assert.Equal(t, "", rr.Header().Get("Access-Control-Allow-Credentials"), "Expected no credentials header")
}

// TestCORSMiddleware_PreflightOPTIONS tests preflight OPTIONS request handling
func TestCORSMiddleware_PreflightOPTIONS(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create preflight request
	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "http://localhost:8080", rr.Header().Get("Access-Control-Allow-Origin"), "Expected correct CORS origin header")
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS, PATCH", rr.Header().Get("Access-Control-Allow-Methods"), "Expected correct methods header")
	assert.Equal(t, "Content-Type, Authorization, X-Requested-With", rr.Header().Get("Access-Control-Allow-Headers"), "Expected correct headers header")
	assert.Equal(t, "86400", rr.Header().Get("Access-Control-Max-Age"), "Expected max-age header")
}

// TestCORSMiddleware_PreflightOPTIONS_UnauthorizedOrigin tests preflight with unauthorized origin
func TestCORSMiddleware_PreflightOPTIONS_UnauthorizedOrigin(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create preflight request with unauthorized origin
	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "", rr.Header().Get("Access-Control-Allow-Origin"), "Expected no CORS origin header")
	assert.Equal(t, "", rr.Header().Get("Access-Control-Allow-Methods"), "Expected no methods header")
}

// TestCORSMiddleware_MultipleAllowedOrigins tests multiple allowed origins
func TestCORSMiddleware_MultipleAllowedOrigins(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080", "https://example.com", "https://test.com"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Test each allowed origin
	testCases := []string{"http://localhost:8080", "https://example.com", "https://test.com"}

	for _, origin := range testCases {
		// Create request with origin
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Origin", origin)

		// Act
		rr := httptest.NewRecorder()
		handler := corsMiddleware.Handler(nextHandler)
		handler.ServeHTTP(rr, req)

		// Assert
		assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 for origin "+origin)
		assert.Equal(t, origin, rr.Header().Get("Access-Control-Allow-Origin"), "Expected correct CORS origin header for "+origin)
		assert.Equal(t, "true", rr.Header().Get("Access-Control-Allow-Credentials"), "Expected credentials header for "+origin)
	}
}

// TestCORSMiddleware_NoOriginHeader tests request without Origin header
func TestCORSMiddleware_NoOriginHeader(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request without Origin header
	req := httptest.NewRequest("GET", "/api/test", nil)

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "", rr.Header().Get("Access-Control-Allow-Origin"), "Expected no CORS origin header")
}

// TestCORSMiddleware_WithPorts tests origins with ports
func TestCORSMiddleware_WithPorts(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080", "https://example.com:443", "http://test.com:3000"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with origin that has a port
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "http://localhost:8080", rr.Header().Get("Access-Control-Allow-Origin"), "Expected correct CORS origin header with port")
}

// TestCORSMiddleware_CaseSensitiveOrigin tests case-sensitive origin matching
func TestCORSMiddleware_CaseSensitiveOrigin(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with different case origin
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://LOCALHOST:8080")

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert - should NOT match due to case sensitivity
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "", rr.Header().Get("Access-Control-Allow-Origin"), "Expected no CORS origin header (case mismatch)")
}

// TestCORSMiddleware_EmptyAllowedOrigins tests empty allowed origins list
func TestCORSMiddleware_EmptyAllowedOrigins(t *testing.T) {
	// Arrange
	allowedOrigins := []string{}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with origin
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")

	// Act
	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "", rr.Header().Get("Access-Control-Allow-Origin"), "Expected no CORS origin header (empty allowed list)")
}

// TestCORSMiddleware_VaryOriginHeader tests that Vary: Origin is set for allowed origins
func TestCORSMiddleware_VaryOriginHeader(t *testing.T) {
	// Arrange
	allowedOrigins := []string{"http://localhost:8080"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(allowedOrigins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test regular request with allowed origin
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")

	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "Origin", rr.Header().Get("Vary"), "Expected Vary: Origin header for allowed origin")

	// Test preflight request with allowed origin
	req = httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "Origin", rr.Header().Get("Vary"), "Expected Vary: Origin header on preflight")
}

// TestCORSMiddleware_DefensiveCopy tests that middleware is not affected by external slice mutation
func TestCORSMiddleware_DefensiveCopy(t *testing.T) {
	// Arrange
	origins := []string{"http://localhost:8080"}
	corsMiddleware := appmiddleware.NewCORSMiddleware(origins, testLogger())

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mutate the original slice after middleware creation
	origins[0] = "http://evil.com"

	// Act - request with original origin should still work
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")

	rr := httptest.NewRecorder()
	handler := corsMiddleware.Handler(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert - defensive copy prevents external mutation
	assert.Equal(t, "http://localhost:8080", rr.Header().Get("Access-Control-Allow-Origin"), "Expected original origin to still be allowed after slice mutation")
}
