package modgen_test

import (
	"testing"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsKnownSurface(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		wantErr  bool
	}{
		{
			name:     "success - routes",
			input:    "routes",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "success - trims whitespace",
			input:    "  migrations  ",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "success - unknown",
			input:    "hooks",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "success - empty",
			input:    "",
			expected: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, modgen.IsKnownSurface(tt.input))
			require.Equal(t, tt.wantErr, false)
		})
	}
}

func TestKnownSurfaces(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - canonical list",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			surfaces := modgen.KnownSurfaces()
			require.Equal(t, tt.wantErr, false)
			assert.Contains(t, surfaces, "routes")
			assert.Contains(t, surfaces, "public_routes")
			assert.Contains(t, surfaces, "migrations")
			assert.Contains(t, surfaces, "posttypes")
			assert.Contains(t, surfaces, "roles")
			assert.Contains(t, surfaces, "themes")
			assert.Contains(t, surfaces, "admin")
		})
	}
}
