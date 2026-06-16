package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdminMiddleware_AdminOnly_ValidAdminToken tests admin middleware with valid admin token
func TestAdminMiddleware_AdminOnly_ValidAdminToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := appmiddleware.NewAdminMiddleware(authMiddleware)

	// Generate a token with Admin role
	token, err := jwtManager.GenerateToken("123", "admin", "Admin")
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with admin token
	req := httptest.NewRequest("GET", "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := adminMiddleware.AdminOnly(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "OK", rr.Body.String(), "Expected body 'OK'")
}

// TestAdminMiddleware_AdminOnly_NonAdminToken tests admin middleware with non-admin token
func TestAdminMiddleware_AdminOnly_NonAdminToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := appmiddleware.NewAdminMiddleware(authMiddleware)

	// Generate a token with Contributor role
	token, err := jwtManager.GenerateToken("123", "contributor", "Contributor")
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with contributor token trying to access admin endpoint
	req := httptest.NewRequest("GET", "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := adminMiddleware.AdminOnly(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected status 403")

	// Verify error response
	body := rr.Body.String()
	assert.NotEmpty(t, body, "Expected error response")
	assert.NotEqual(t, "OK", body, "Expected error response, not success")
}

// TestAdminMiddleware_AdminOnly_MissingToken tests admin middleware with missing token
func TestAdminMiddleware_AdminOnly_MissingToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := appmiddleware.NewAdminMiddleware(authMiddleware)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request without token
	req := httptest.NewRequest("GET", "/admin/users", nil)

	// Act
	rr := httptest.NewRecorder()
	handler := adminMiddleware.AdminOnly(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401")
}

// TestAdminMiddleware_AdminOnly_InvalidToken tests admin middleware with invalid token
func TestAdminMiddleware_AdminOnly_InvalidToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := appmiddleware.NewAdminMiddleware(authMiddleware)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with invalid token
	req := httptest.NewRequest("GET", "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-123")

	// Act
	rr := httptest.NewRecorder()
	handler := adminMiddleware.AdminOnly(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401")
}

// TestAdminMiddleware_AdminOnly_EditorRole tests admin middleware with Editor role
func TestAdminMiddleware_AdminOnly_EditorRole(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := appmiddleware.NewAdminMiddleware(authMiddleware)

	// Generate a token with Editor role
	token, err := jwtManager.GenerateToken("123", "editor", "Editor")
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with editor token trying to access admin endpoint
	req := httptest.NewRequest("GET", "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := adminMiddleware.AdminOnly(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected status 403")
}

// TestAdminMiddleware_AdminOnly_UserRole tests admin middleware with regular User role
func TestAdminMiddleware_AdminOnly_UserRole(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	authMiddleware := appmiddleware.NewAuthMiddleware(jwtManager)
	adminMiddleware := appmiddleware.NewAdminMiddleware(authMiddleware)

	// Generate a token with User role
	token, err := jwtManager.GenerateToken("123", "user", "User")
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with user token trying to access admin endpoint
	req := httptest.NewRequest("GET", "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := adminMiddleware.AdminOnly(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected status 403")
}
