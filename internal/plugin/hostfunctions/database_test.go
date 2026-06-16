package hostfunctions

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTableName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "content_items maps to content",
			input:    "content_items",
			expected: "content",
		},
		{
			name:     "media_files maps to media",
			input:    "media_files",
			expected: "media",
		},
		{
			name:     "users stays unchanged",
			input:    "users",
			expected: "users",
		},
		{
			name:     "unknown table stays unchanged",
			input:    "orders",
			expected: "orders",
		},
		{
			name:     "empty string stays unchanged",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeTableName(tt.input))
		})
	}
}

func TestCheckDBPermission(t *testing.T) {
	tests := []struct {
		name       string
		manifest   *capability.Manifest
		query      string
		operation  string
		wantErr    bool
		errContain string
	}{
		{
			name: "read content_items allowed with read:content",
			manifest: &capability.Manifest{
				Capabilities: capability.Capabilities{
					Database: []string{"read:content"},
				},
			},
			query:     "SELECT * FROM content_items WHERE id = 1",
			operation: "read",
			wantErr:   false,
		},
		{
			name: "read media_files allowed with read:media",
			manifest: &capability.Manifest{
				Capabilities: capability.Capabilities{
					Database: []string{"read:media"},
				},
			},
			query:     "SELECT * FROM media_files WHERE id = 1",
			operation: "read",
			wantErr:   false,
		},
		{
			name: "read users allowed with read:users",
			manifest: &capability.Manifest{
				Capabilities: capability.Capabilities{
					Database: []string{"read:users"},
				},
			},
			query:     "SELECT custom_fields FROM users WHERE id = 1",
			operation: "read",
			wantErr:   false,
		},
		{
			name: "write users allowed with write:users",
			manifest: &capability.Manifest{
				Capabilities: capability.Capabilities{
					Database: []string{"write:users"},
				},
			},
			query:     "UPDATE users SET custom_fields = ? WHERE id = 1",
			operation: "write",
			wantErr:   false,
		},
		{
			name: "write content_items allowed with write:content",
			manifest: &capability.Manifest{
				Capabilities: capability.Capabilities{
					Database: []string{"write:content"},
				},
			},
			query:     "UPDATE content_items SET title = ? WHERE id = 1",
			operation: "write",
			wantErr:   false,
		},
		{
				name: "write content_items allowed with write:content",
				manifest: &capability.Manifest{
					Capabilities: capability.Capabilities{
						Database: []string{"write:content"},
					},
				},
				query:     "UPDATE content_items SET custom_fields = ? WHERE id = 1",
				operation: "write",
				wantErr:   false,
			},
		{
			name: "read denied without matching permission",
			manifest: &capability.Manifest{
				Capabilities: capability.Capabilities{
					Database: []string{"read:users"},
				},
			},
			query:      "SELECT * FROM content_items WHERE id = 1",
			operation: "read",
			wantErr:    true,
			errContain: "permission",
		},
		{
			name: "write denied without matching permission",
			manifest: &capability.Manifest{
				Capabilities: capability.Capabilities{
					Database: []string{"read:content"},
				},
			},
			query:      "UPDATE users SET custom_fields = ? WHERE id = 1",
			operation: "write",
			wantErr:    true,
			errContain: "permission",
		},
		{
			name:       "unrecognizable query returns error",
			manifest:   &capability.Manifest{},
			query:      "BOGUS QUERY",
			operation:  "read",
			wantErr:    true,
			errContain: "could not determine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDBPermission(tt.manifest, tt.query, tt.operation)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}
