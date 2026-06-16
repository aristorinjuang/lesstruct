package capability_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest(t *testing.T) {
	t.Run("returns nil for missing file", func(t *testing.T) {
		m, err := capability.LoadManifest("/nonexistent/path.manifest")
		require.NoError(t, err)
		assert.Nil(t, m)
	})

	t.Run("parses a valid manifest", func(t *testing.T) {
		path := writeTempManifest(t, `
name = "test-plugin"
version = "1.0.0"

[capabilities]
http = ["https://api.example.com/*"]
database = ["read:content"]
`)

		m, err := capability.LoadManifest(path)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.Equal(t, "test-plugin", m.Name)
		assert.Equal(t, "1.0.0", m.Version)
		assert.True(t, m.HasHTTP())
		assert.True(t, m.HasDatabase())
		assert.Equal(t, []string{"https://api.example.com/*"}, m.Capabilities.HTTP)
		assert.Equal(t, []string{"read:content"}, m.Capabilities.Database)
	})

	t.Run("parses manifest with rate limit", func(t *testing.T) {
		path := writeTempManifest(t, `
name = "rated"
version = "2.0.0"

[capabilities.rate_limit]
requests_per_minute = 60
`)

		m, err := capability.LoadManifest(path)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.Equal(t, 60, m.Capabilities.RateLimit.RequestsPerMinute)
	})

	t.Run("parses manifest with max memory", func(t *testing.T) {
		path := writeTempManifest(t, `
name = "mem"
version = "1.0.0"
max_memory_mb = 128
`)

		m, err := capability.LoadManifest(path)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.Equal(t, 128, m.MaxMemoryMB)
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		path := writeTempManifest(t, `
name = ""
version = "1.0.0"
`)

		_, err := capability.LoadManifest(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("returns error for empty version", func(t *testing.T) {
		path := writeTempManifest(t, `
name = "test"
version = ""
`)

		_, err := capability.LoadManifest(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "version is required")
	})

	t.Run("returns error for invalid database permission", func(t *testing.T) {
		path := writeTempManifest(t, `
name = "bad"
version = "1.0.0"

[capabilities]
database = ["read:passwords"]
`)

		_, err := capability.LoadManifest(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown database permission")
	})

	t.Run("accepts write:users database permission", func(t *testing.T) {
		path := writeTempManifest(t, `
name = "user-updater"
version = "1.0.0"

[capabilities]
database = ["write:users"]
`)

		m, err := capability.LoadManifest(path)
		require.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, []string{"write:users"}, m.Capabilities.Database)
	})

	t.Run("manifest without capabilities has no http or db", func(t *testing.T) {
		path := writeTempManifest(t, `
name = "minimal"
version = "1.0.0"
`)

		m, err := capability.LoadManifest(path)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.False(t, m.HasHTTP())
		assert.False(t, m.HasDatabase())
	})
}

func TestManifestIsHTTPURLAllowed(t *testing.T) {
	m := capability.Manifest{
		Name:    "test",
		Version: "1.0.0",
		Capabilities: capability.Capabilities{
			HTTP: []string{
				"https://api.example.com/*",
				"https://exact.url/path",
			},
		},
	}

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "matched by wildcard pattern",
			url:      "https://api.example.com/v1/data",
			expected: true,
		},
		{
			name:     "matched by exact pattern",
			url:      "https://exact.url/path",
			expected: true,
		},
		{
			name:     "not matched",
			url:      "https://evil.com/steal",
			expected: false,
		},
		{
			name:     "partial prefix not matched",
			url:      "https://api.example.co/evil",
			expected: false,
		},
		{
			name:     "empty url",
			url:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, m.IsHTTPURLAllowed(tt.url))
		})
	}
}

func TestManifestHasDBPermission(t *testing.T) {
	m := capability.Manifest{
		Name:    "test",
		Version: "1.0.0",
		Capabilities: capability.Capabilities{
			Database: []string{"read:content", "write:media"},
		},
	}

	tests := []struct {
		name     string
		perm     string
		expected bool
	}{
		{
			name:     "allow listed permission",
			perm:     "read:content",
			expected: true,
		},
		{
			name:     "allow listed write permission",
			perm:     "write:media",
			expected: true,
		},
		{
			name:     "reject unlisted permission",
			perm:     "read:users",
			expected: false,
		},
		{
			name:     "reject empty string",
			perm:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, m.HasDBPermission(tt.perm))
		})
	}
}

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name     string
		manifest capability.Manifest
		wantErr  bool
	}{
		{
			name: "valid",
			manifest: capability.Manifest{
				Name:    "test",
				Version: "1.0.0",
			},
			wantErr: false,
		},
		{
			name:     "empty name",
			manifest: capability.Manifest{Name: "", Version: "1.0.0"},
			wantErr:  true,
		},
		{
			name:     "empty version",
			manifest: capability.Manifest{Name: "test", Version: ""},
			wantErr:  true,
		},
		{
			name: "bad db permission",
			manifest: capability.Manifest{
				Name:    "test",
				Version: "1.0.0",
				Capabilities: capability.Capabilities{
					Database: []string{"invalid:permission"},
				},
			},
			wantErr: true,
		},
		{
			name: "negative max memory",
			manifest: capability.Manifest{
				Name:        "test",
				Version:     "1.0.0",
				MaxMemoryMB: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func writeTempManifest(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.manifest")
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)

	return path
}