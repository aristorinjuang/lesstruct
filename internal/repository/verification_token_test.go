package repository_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t *testing.T) {
	token, err := authdomain.GenerateToken()

	require.NoError(t, err, "GenerateToken() failed")
	assert.NotEmpty(t, token, "Token should not be empty")
	assert.Equal(t, 64, len(token), "Token length should be 64 (32 bytes * 2 for hex encoding)")

	// Generate another token and ensure they're different
	token2, err := authdomain.GenerateToken()
	require.NoError(t, err, "GenerateToken() failed")
	assert.NotEqual(t, token, token2, "Tokens should be unique")
}

func TestHashToken(t *testing.T) {
	token := "test-token-123"
	hash := authdomain.HashToken(token)

	assert.NotEmpty(t, hash, "Hash should not be empty")
	assert.Equal(t, 64, len(hash), "Hash length should be 64 (SHA-256)")

	// Same token should produce same hash
	hash2 := authdomain.HashToken(token)
	assert.Equal(t, hash, hash2, "Same token should produce same hash")

	// Different tokens should produce different hashes
	differentToken := "different-token-456"
	differentHash := authdomain.HashToken(differentToken)
	assert.NotEqual(t, hash, differentHash, "Different tokens should produce different hashes")
}

func TestVerificationToken_Interface(t *testing.T) {
	// Ensure the repository implements the interface
	var _ authdomain.VerificationTokenRepo = &repository.VerificationTokenRepository{}
}

func TestPasswordResetToken_Interface(t *testing.T) {
	// Ensure the repository implements the interface
	var _ authdomain.PasswordResetTokenRepo = &repository.PasswordResetTokenRepository{}
}
