package kernel_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrincipalHasRole(t *testing.T) {
	tests := []struct {
		name      string
		principal kernel.Principal
		role      string
		expected  bool
	}{
		{
			name:      "has role",
			principal: kernel.Principal{ID: "1", Username: "ari", Roles: []string{"admin", "editor"}},
			role:      "admin",
			expected:  true,
		},
		{
			name:      "missing role",
			principal: kernel.Principal{ID: "1", Username: "ari", Roles: []string{"editor"}},
			role:      "admin",
			expected:  false,
		},
		{
			name:      "no roles",
			principal: kernel.Principal{ID: "1", Username: "ari"},
			role:      "admin",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.principal.HasRole(tt.role))
		})
	}
}

func TestPrincipalIsAnonymous(t *testing.T) {
	tests := []struct {
		name      string
		principal kernel.Principal
		expected  bool
	}{
		{
			name:      "empty principal is anonymous",
			principal: kernel.Principal{},
			expected:  true,
		},
		{
			name:      "identified principal is not anonymous",
			principal: kernel.Principal{ID: "1", Username: "ari", Roles: []string{"admin"}},
			expected:  false,
		},
		{
			name:      "username alone is not anonymous",
			principal: kernel.Principal{Username: "ari"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.principal.IsAnonymous())
		})
	}
}

func TestPrincipalContextRoundTrip(t *testing.T) {
	principal := kernel.Principal{ID: "7", Username: "ari", Roles: []string{"admin"}}
	ctx := kernel.ContextWithPrincipal(context.Background(), principal)

	got, ok := kernel.PrincipalFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, principal, got)
}

func TestPrincipalFromContextMissing(t *testing.T) {
	_, ok := kernel.PrincipalFromContext(context.Background())
	assert.False(t, ok)
}
