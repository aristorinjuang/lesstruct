package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aristorinjuang/lesstruct/internal/config"
)

func TestLoadLanguages(t *testing.T) {
	tests := []struct {
		name      string
		toml      string
		expected  []string
		expectErr bool
	}{
		{
			name:     "success - multiple languages from toml",
			toml:     `languages = ["en", "id", "fr"]` + "\n",
			expected: []string{"en", "id", "fr"},
			expectErr: false,
		},
		{
			name:     "success - single language",
			toml:     `languages = ["en"]` + "\n",
			expected: []string{"en"},
			expectErr: false,
		},
		{
			name:     "success - empty languages falls back to en",
			toml:     `languages = []` + "\n",
			expected: []string{"en"},
			expectErr: false,
		},
		{
			name:     "success - no languages key falls back to en",
			toml:     `post_type = []` + "\n",
			expected: []string{"en"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.toml")
			err := os.WriteFile(configPath, []byte(tt.toml), 0644)
			require.NoError(t, err)

			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
			}

			result, err := config.LoadLanguages(cfg)

			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadLanguages_MissingConfigFile(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "/nonexistent/path",
		ConfigFile: "config.toml",
	}

	languages, err := config.LoadLanguages(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"en"}, languages)
}

func TestLoadLanguages_NilConfig(t *testing.T) {
	languages, err := config.LoadLanguages(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"en"}, languages)
}

func TestPrimaryLanguage(t *testing.T) {
	tests := []struct {
		name      string
		languages []string
		expected  string
	}{
		{
			name:      "success - multiple languages returns first",
			languages: []string{"en", "id", "fr"},
			expected:  "en",
		},
		{
			name:      "success - single language",
			languages: []string{"id"},
			expected:  "id",
		},
		{
			name:      "success - empty slice defaults to en",
			languages: []string{},
			expected:  "en",
		},
		{
			name:      "success - nil slice defaults to en",
			languages: nil,
			expected:  "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.PrimaryLanguage(tt.languages)
			assert.Equal(t, tt.expected, result)
		})
	}
}
