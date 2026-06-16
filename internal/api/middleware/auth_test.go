package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMiddleware_RequireAuth_ValidToken tests authentication with valid token
func TestAuthMiddleware_RequireAuth_ValidToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	// Generate a valid token
	token, err := jwtManager.GenerateToken("123", "testuser", "Contributor")
	require.NoError(t, err, "Failed to generate token")

	// Create a test handler that checks user context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := appmiddleware.GetUserID(r)
		require.True(t, ok, "Expected user ID in context")

		username, ok := appmiddleware.GetUsername(r)
		require.True(t, ok, "Expected username in context")

		role, ok := appmiddleware.GetRole(r)
		require.True(t, ok, "Expected role in context")

		assert.Equal(t, "123", userID, "Expected user ID '123'")
		assert.Equal(t, "testuser", username, "Expected username 'testuser'")
		assert.Equal(t, "Contributor", role, "Expected role 'Contributor'")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with valid token
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := middleware.RequireAuth(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
	assert.Equal(t, "OK", rr.Body.String(), "Expected body 'OK'")
}

// TestAuthMiddleware_RequireAuth_MissingToken tests authentication with missing token
func TestAuthMiddleware_RequireAuth_MissingToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request without token
	req := httptest.NewRequest("GET", "/protected", nil)

	// Act
	rr := httptest.NewRecorder()
	handler := middleware.RequireAuth(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401")
}

// TestAuthMiddleware_RequireAuth_InvalidToken tests authentication with invalid token
func TestAuthMiddleware_RequireAuth_InvalidToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with invalid token
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-123")

	// Act
	rr := httptest.NewRecorder()
	handler := middleware.RequireAuth(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401")
}

// TestAuthMiddleware_RequireAuth_MalformedHeader tests authentication with malformed Authorization header
func TestAuthMiddleware_RequireAuth_MalformedHeader(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	tests := []struct {
		name       string
		authHeader string
	}{
		{
			name:       "No Bearer prefix",
			authHeader: "invalid-token-123",
		},
		{
			name:       "Empty Bearer token",
			authHeader: "Bearer ",
		},
		{
			name:       "Only Bearer prefix",
			authHeader: "Bearer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with malformed header
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", tt.authHeader)

			// Act
			rr := httptest.NewRecorder()
			handler := middleware.RequireAuth(nextHandler)
			handler.ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401")
		})
	}
}

// TestAuthMiddleware_OptionalAuth_ValidToken tests optional authentication with valid token
func TestAuthMiddleware_OptionalAuth_ValidToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	// Generate a valid token
	token, err := jwtManager.GenerateToken("123", "testuser", "Contributor")
	require.NoError(t, err, "Failed to generate token")

	// Create a test handler that checks user context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := appmiddleware.GetUserID(r)
		require.True(t, ok, "Expected user ID in context")

		assert.Equal(t, "123", userID, "Expected user ID '123'")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with valid token
	req := httptest.NewRequest("GET", "/public", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := middleware.OptionalAuth(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
}

// TestAuthMiddleware_OptionalAuth_NoToken tests optional authentication without token
func TestAuthMiddleware_OptionalAuth_NoToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that user context is NOT set
		_, ok := appmiddleware.GetUserID(r)
		assert.False(t, ok, "Expected no user ID in context when no token provided")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request without token
	req := httptest.NewRequest("GET", "/public", nil)

	// Act
	rr := httptest.NewRecorder()
	handler := middleware.OptionalAuth(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
}

// TestAuthMiddleware_RequireRole_ValidRole tests role-based access control with valid role
func TestAuthMiddleware_RequireRole_ValidRole(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	// Generate a token with Admin role
	token, err := jwtManager.GenerateToken("123", "admin", "Admin")
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with admin token
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := middleware.RequireRole("Admin", "Editor")(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")
}

// TestAuthMiddleware_RequireRole_InvalidRole tests role-based access control with invalid role
func TestAuthMiddleware_RequireRole_InvalidRole(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager)

	// Generate a token with Contributor role
	token, err := jwtManager.GenerateToken("123", "testuser", "Contributor")
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with contributor token trying to access admin endpoint
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Act
	rr := httptest.NewRecorder()
	handler := middleware.RequireRole("Admin", "Editor")(nextHandler)
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected status 403")
}

// TestGetUserID tests extracting user ID from context
func TestGetUserID(t *testing.T) {
	// Arrange
	ctx := context.WithValue(context.Background(), appmiddleware.UserIDKey, "123")

	// Create request with context
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ctx)

	// Act
	userID, ok := appmiddleware.GetUserID(req)

	// Assert
	assert.True(t, ok, "Expected ok to be true")
	assert.Equal(t, "123", userID, "Expected user ID '123'")
}

// TestGetUsername tests extracting username from context
func TestGetUsername(t *testing.T) {
	// Arrange
	ctx := context.WithValue(context.Background(), appmiddleware.UsernameKey, "testuser")

	// Create request with context
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ctx)

	// Act
	username, ok := appmiddleware.GetUsername(req)

	// Assert
	assert.True(t, ok, "Expected ok to be true")
	assert.Equal(t, "testuser", username, "Expected username 'testuser'")
}

// TestGetRole tests extracting role from context
func TestGetRole(t *testing.T) {
	// Arrange
	ctx := context.WithValue(context.Background(), appmiddleware.RoleKey, "Admin")

	// Create request with context
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ctx)

	// Act
	role, ok := appmiddleware.GetRole(req)

	// Assert
	assert.True(t, ok, "Expected ok to be true")
	assert.Equal(t, "Admin", role, "Expected role 'Admin'")
}
