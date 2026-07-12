package auth_test

import (
	"testing"

	authpkg "github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/domain/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFirstLoginService(t *testing.T) {
	service := auth.NewFirstLoginService()
	require.NotNil(t, service, "NewFirstLoginService() should not return nil")
}

func TestIsSetupComplete(t *testing.T) {
	defaultHash, err := authpkg.HashPassword(constants.DefaultPassword)
	require.NoError(t, err, "Failed to hash default password")

	changedHash, err := authpkg.HashPassword("D1fferent!Password")
	require.NoError(t, err, "Failed to hash changed password")

	tests := []struct {
		name     string
		hash     string
		expected bool
	}{
		{
			name:     "not complete - admin still uses default password",
			hash:     defaultHash,
			expected: false,
		},
		{
			name:     "complete - admin password has been changed",
			hash:     changedHash,
			expected: true,
		},
		{
			name:     "not complete - empty hash",
			hash:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := auth.NewFirstLoginService()
			assert.Equal(t, tt.expected, service.IsSetupComplete(tt.hash))
		})
	}
}
