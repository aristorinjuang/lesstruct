package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommentatorMiddleware_CommentatorOnly tests that Commentator role can access protected endpoints
func TestCommentatorMiddleware_CommentatorOnly(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		expectedStatus int
	}{
		{
			name:           "Commentator can access",
			role:           RoleCommentator,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Contributor can access",
			role:           RoleContributor,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Admin can access",
			role:           RoleAdmin,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Unauthenticated user cannot access",
			role:           "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid role cannot access",
			role:           "InvalidRole",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
			authMiddleware := NewAuthMiddleware(jwtManager)
			commentatorMiddleware := NewCommentatorMiddleware(authMiddleware)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			req := httptest.NewRequest("GET", "/protected", nil)

			if tt.role != "" {
				token, err := jwtManager.GenerateToken("123", "testuser", tt.role)
				require.NoError(t, err, "Failed to generate token")
				req.Header.Set("Authorization", "Bearer "+token)
			}

			rr := httptest.NewRecorder()
			handler := commentatorMiddleware.CommentatorOnly(nextHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, "Expected status %d", tt.expectedStatus)
		})
	}
}

// TestCommentatorMiddleware_RoleHierarchy tests that higher roles can access Commentator endpoints
func TestCommentatorMiddleware_RoleHierarchy(t *testing.T) {
	jwtManager := appauth.NewJWTManager("test-secret-key-for-testing-purposes-min-32-chars")
	authMiddleware := NewAuthMiddleware(jwtManager)
	commentatorMiddleware := NewCommentatorMiddleware(authMiddleware)

	roles := []string{RoleCommentator, RoleContributor, RoleAdmin}

	for _, role := range roles {
		t.Run(role+" can access Commentator endpoint", func(t *testing.T) {
			token, err := jwtManager.GenerateToken("123", "testuser", role)
			require.NoError(t, err, "Failed to generate token")

			req := httptest.NewRequest("GET", "/comments", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			rr := httptest.NewRecorder()
			handler := commentatorMiddleware.CommentatorOnly(nextHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 for role %s", role)
		})
	}
}

// TestRoleConstants tests that role constants are properly defined
func TestRoleConstants(t *testing.T) {
	assert.Equal(t, "Admin", RoleAdmin, "RoleAdmin should be 'Admin'")
	assert.Equal(t, "Contributor", RoleContributor, "RoleContributor should be 'Contributor'")
	assert.Equal(t, "Commentator", RoleCommentator, "RoleCommentator should be 'Commentator'")

	assert.Equal(t, RoleAdmin, constants.RoleAdmin, "Admin role constants should match")
	assert.Equal(t, RoleContributor, constants.RoleContributor, "Contributor role constants should match")
	assert.Equal(t, RoleCommentator, constants.RoleCommentator, "Commentator role constants should match")
}
