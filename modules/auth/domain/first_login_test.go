package auth_test

import (
	"testing"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewFirstLoginService(t *testing.T) {
	service := authdomain.NewFirstLoginService(stubPolicy{})
	assert.NotNil(t, service, "NewFirstLoginService() should not return nil")
}

func TestIsSetupComplete(t *testing.T) {
	policy := stubPolicy{}
	defaultHash, err := policy.HashPassword(authdomain.DefaultPassword)
	if err != nil {
		t.Fatalf("Failed to hash default password: %v", err)
	}

	changedHash, err := policy.HashPassword("D1fferent!Password")
	if err != nil {
		t.Fatalf("Failed to hash changed password: %v", err)
	}

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
			service := authdomain.NewFirstLoginService(stubPolicy{})
			assert.Equal(t, tt.expected, service.IsSetupComplete(tt.hash))
		})
	}
}
