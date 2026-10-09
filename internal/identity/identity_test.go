package identity_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/identity"
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		userID   int
		username string
		role     string
		expected kernel.Principal
	}{
		{
			name:     "success - full identity",
			userID:   42,
			username: "alice",
			role:     "Admin",
			expected: kernel.Principal{ID: "42", Username: "alice", Roles: []string{"Admin"}},
		},
		{
			name:     "success - empty role omitted",
			userID:   7,
			username: "anon",
			role:     "",
			expected: kernel.Principal{ID: "7", Username: "anon", Roles: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, identity.New(tt.userID, tt.username, tt.role))
		})
	}
}

func TestIntID(t *testing.T) {
	tests := []struct {
		name      string
		principal kernel.Principal
		expected  int
		wantErr   bool
	}{
		{
			name:      "success - decimal id",
			principal: kernel.Principal{ID: "42", Username: "alice", Roles: []string{"Admin"}},
			expected:  42,
			wantErr:   false,
		},
		{
			name:      "error - non-decimal id",
			principal: kernel.Principal{ID: "abc", Username: "alice"},
			wantErr:   true,
		},
		{
			name:      "error - empty id",
			principal: kernel.Principal{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := identity.IntID(tt.principal)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, identity.ErrInvalidUserID)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, id)
		})
	}
}

func TestPrimaryRole(t *testing.T) {
	tests := []struct {
		name      string
		principal kernel.Principal
		expected  string
	}{
		{
			name:      "success - first role",
			principal: kernel.Principal{ID: "1", Roles: []string{"Admin", "Editor"}},
			expected:  "Admin",
		},
		{
			name:      "success - anonymous has none",
			principal: kernel.Principal{},
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, identity.PrimaryRole(tt.principal))
		})
	}
}
