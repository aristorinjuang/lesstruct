package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appmiddleware "github.com/aristorinjuang/lesstruct/internal/api/middleware"
	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMiddleware_RequireAuth_ValidToken tests authentication with valid token
func TestAuthMiddleware_RequireAuth_ValidToken(t *testing.T) {
	// Arrange
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	// Generate a valid token
	token, err := jwtManager.GenerateToken("123", "testuser", "Contributor", 0)
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
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
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
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request without token
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)

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
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with invalid token
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
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
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

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
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
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
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	// Generate a valid token
	token, err := jwtManager.GenerateToken("123", "testuser", "Contributor", 0)
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
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
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
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that user context is NOT set
		_, ok := appmiddleware.GetUserID(r)
		assert.False(t, ok, "Expected no user ID in context when no token provided")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request without token
	req := httptest.NewRequest(http.MethodGet, "/public", nil)

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
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	// Generate a token with Admin role
	token, err := jwtManager.GenerateToken("123", "admin", "Admin", 0)
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with admin token
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
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
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	// Generate a token with Contributor role
	token, err := jwtManager.GenerateToken("123", "testuser", "Contributor", 0)
	require.NoError(t, err, "Failed to generate token")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Create request with contributor token trying to access admin endpoint
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ctx)

	// Act
	role, ok := appmiddleware.GetRole(req)

	// Assert
	assert.True(t, ok, "Expected ok to be true")
	assert.Equal(t, "Admin", role, "Expected role 'Admin'")
}

// TestPrincipalInjected verifies both auth paths attach the kernel identity.
func TestPrincipalInjected(t *testing.T) {
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

	adminToken, err := jwtManager.GenerateToken("7", "alice", "Admin", 0)
	require.NoError(t, err, "Failed to generate token")

	tests := []struct {
		name         string
		handler      func(http.Handler) http.Handler
		header       string
		wantPresent  bool
		wantID       string
		wantUsername string
		wantRoles    []string
	}{
		{
			name:         "success - RequireAuth attaches principal",
			handler:      middleware.RequireAuth,
			header:       "Bearer " + adminToken,
			wantPresent:  true,
			wantID:       "7",
			wantUsername: "alice",
			wantRoles:    []string{"Admin"},
		},
		{
			name:        "anonymous - OptionalAuth without token has no principal",
			handler:     middleware.OptionalAuth,
			header:      "",
			wantPresent: false,
		},
		{
			name:         "success - RequireRole attaches principal",
			handler:      middleware.RequireRole("Admin"),
			header:       "Bearer " + adminToken,
			wantPresent:  true,
			wantID:       "7",
			wantUsername: "alice",
			wantRoles:    []string{"Admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotID       string
				gotUsername string
				gotRoles    []string
				gotPresent  bool
			)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				principal, ok := appmiddleware.Principal(r)
				gotPresent = ok
				gotID = principal.ID
				gotUsername = principal.Username
				gotRoles = principal.Roles
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			rr := httptest.NewRecorder()
			tt.handler(nextHandler).ServeHTTP(rr, req)
			require.Equal(t, http.StatusOK, rr.Code)

			assert.Equal(t, tt.wantPresent, gotPresent)
			if tt.wantPresent {
				assert.Equal(t, tt.wantID, gotID)
				assert.Equal(t, tt.wantUsername, gotUsername)
				assert.Equal(t, tt.wantRoles, gotRoles)
			}
		})
	}
}

// TestPrincipalAbsent verifies Principal reports anonymous when no auth ran.
func TestPrincipalAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, ok := appmiddleware.Principal(req)

	assert.False(t, ok, "Expected no principal without auth middleware")
}

func TestAuthMiddleware_APIKeyShapedTokenReturnsJWTRequired(t *testing.T) {
	tests := []struct {
		name  string
		realm string
		token string
	}{
		{
			name:  "success - RequireAuth rejects API key with JWT_REQUIRED",
			realm: "auth",
			token: "Bearer lesstruct_abc123def456_secretsecretsecretsecret12",
		},
		{
			name:  "success - RequireRole rejects API key with JWT_REQUIRED",
			realm: "role",
			token: "Bearer lesstruct_abc123def456_secretsecretsecretsecret12",
		},
		{
			name:  "success - JWT-shaped bad token still returns INVALID_TOKEN",
			realm: "auth",
			token: "Bearer invalid-token-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
			middleware := appmiddleware.NewAuthMiddleware(jwtManager, stubUserLookup{})

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			var handler http.Handler
			if tt.realm == "auth" {
				handler = middleware.RequireAuth(nextHandler)
			} else {
				handler = middleware.RequireRole("Admin")(nextHandler)
			}

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tt.token)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusUnauthorized, rr.Code)
			if tt.token == "Bearer invalid-token-123" {
				assert.Contains(t, rr.Body.String(), "INVALID_TOKEN")
			} else {
				assert.Contains(t, rr.Body.String(), "JWT_REQUIRED")
			}
		})
	}
}
