package plugingen_test

import (
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
)

func TestIsKnownHook(t *testing.T) {
	tests := []struct {
		name     string
		hook     string
		expected bool
		wantErr  bool
	}{
		{
			name:     "success - known hook",
			hook:     "before_save",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "success - unknown hook",
			hook:     "nope",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "success - trimmed hook",
			hook:     "  after_create  ",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "success - empty hook",
			hook:     "",
			expected: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, plugingen.IsKnownHook(tt.hook))
		})
	}
}

func TestIsKnownCapability(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   bool
		wantErr    bool
	}{
		{
			name:       "success - known capability",
			capability: "http_get",
			expected:   true,
			wantErr:    false,
		},
		{
			name:       "success - unknown capability",
			capability: "exec",
			expected:   false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, plugingen.IsKnownCapability(tt.capability))
		})
	}
}

func TestIsKnownDBPermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		expected   bool
		wantErr    bool
	}{
		{
			name:       "success - known permission",
			permission: "read:content",
			expected:   true,
			wantErr:    false,
		},
		{
			name:       "success - unknown permission",
			permission: "read:secrets",
			expected:   false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, plugingen.IsKnownDBPermission(tt.permission))
		})
	}
}

func TestKnownLists(t *testing.T) {
	tests := []struct {
		name    string
		check   func() bool
		wantErr bool
	}{
		{
			name: "success - hooks non-empty",
			check: func() bool {
				return len(plugingen.KnownHooks()) > 0
			},
			wantErr: false,
		},
		{
			name: "success - capabilities non-empty",
			check: func() bool {
				return len(plugingen.KnownCapabilities()) > 0
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.check())
		})
	}
}
