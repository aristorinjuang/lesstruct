package auth_test

import (
	"testing"

	appauth "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t *testing.T) {
	secret := "test-secret-key"
	jwtManager := appauth.NewJWTManager(secret)

	token, err := jwtManager.GenerateToken("user123", "admin", "Admin", 0)
	require.NoError(t, err, "GenerateToken() error")
	assert.NotEmpty(t, token, "GenerateToken() returned empty string")
}

func TestGenerateToken_Validation(t *testing.T) {
	secret := "test-secret-key"
	jwtManager := appauth.NewJWTManager(secret)

	tests := []struct {
		name         string
		userID       string
		username     string
		role         string
		tokenVersion int
		wantErr      bool
	}{
		{
			name:         "Empty user ID",
			userID:       "",
			username:     "admin",
			role:         "Admin",
			tokenVersion: 0,
			wantErr:      true,
		},
		{
			name:         "Empty username",
			userID:       "user123",
			username:     "",
			role:         "Admin",
			tokenVersion: 0,
			wantErr:      true,
		},
		{
			name:         "Empty role",
			userID:       "user123",
			username:     "admin",
			role:         "",
			tokenVersion: 0,
			wantErr:      true,
		},
		{
			name:         "Negative token version",
			userID:       "user123",
			username:     "admin",
			role:         "Admin",
			tokenVersion: -1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jwtManager.GenerateToken(tt.userID, tt.username, tt.role, tt.tokenVersion)
			if tt.wantErr {
				assert.Error(t, err, "GenerateToken() expected error")
			} else {
				assert.NoError(t, err, "GenerateToken() unexpected error")
			}
		})
	}
}

func TestValidateToken_ValidToken(t *testing.T) {
	secret := "test-secret-key"
	jwtManager := appauth.NewJWTManager(secret)

	token, err := jwtManager.GenerateToken("user123", "admin", "Admin", 5)
	require.NoError(t, err, "GenerateToken() error")

	claims, err := jwtManager.ValidateToken(token)
	require.NoError(t, err, "ValidateToken() error")

	assert.Equal(t, "user123", claims.UserID, "UserID")
	assert.Equal(t, "admin", claims.Username, "Username")
	assert.Equal(t, "Admin", claims.Role, "Role")
	assert.Equal(t, 5, claims.TokenVersion, "TokenVersion")
}

func TestValidateToken_InvalidToken(t *testing.T) {
	secret := "test-secret-key"
	jwtManager := appauth.NewJWTManager(secret)

	invalidToken := "invalid.token.string"

	_, err := jwtManager.ValidateToken(invalidToken)
	assert.Error(t, err, "ValidateToken() expected error for invalid token")
}

func TestValidateToken_WrongSecret(t *testing.T) {
	secret1 := "secret-key-1"
	secret2 := "secret-key-2"

	jwtManager1 := appauth.NewJWTManager(secret1)
	jwtManager2 := appauth.NewJWTManager(secret2)

	token, err := jwtManager1.GenerateToken("user123", "admin", "Admin", 0)
	require.NoError(t, err, "GenerateToken() error")

	_, err = jwtManager2.ValidateToken(token)
	assert.Error(t, err, "ValidateToken() expected error for token signed with different secret")
}
