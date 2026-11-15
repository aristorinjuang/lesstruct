package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTheme(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
		wantErr  bool
	}{
		{
			name:     "success - active theme",
			content:  "[theme]\nactive = \"midnight-serif\"\n",
			expected: "midnight-serif",
			wantErr:  false,
		},
		{
			name:     "success - absent block yields empty",
			content:  "[site_config]\nname = \"x\"\n",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "success - trims spaces",
			content:  "[theme]\nactive = \"  midnight  \"\n",
			expected: "midnight",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tt.content), 0o644))
			got, err := config.LoadTheme(cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got.Active)
		})
	}
}

func TestSaveTheme(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		active   string
		expected string
		wantErr  bool
	}{
		{
			name:     "success - creates block",
			initial:  "[site_config]\nname = \"x\"\n",
			active:   "midnight-serif",
			expected: "midnight-serif",
			wantErr:  false,
		},
		{
			name:     "success - clears active",
			initial:  "[theme]\nactive = \"old\"\n",
			active:   "",
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tt.initial), 0o644))
			err := config.SaveTheme(cfg, tt.active)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, err := config.LoadTheme(cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got.Active)
		})
	}
}

func TestThemesRoot(t *testing.T) {
	tests := []struct {
		name     string
		themesDir string
		expected string
	}{
		{
			name:      "success - default",
			themesDir: "",
			expected:  "themes",
		},
		{
			name:      "success - custom",
			themesDir: "custom-themes",
			expected:  "custom-themes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{ThemesDir: tt.themesDir}
			assert.Equal(t, tt.expected, cfg.ThemesRoot())
		})
	}
}

func TestThemeGenerationFlags(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.Config
		enabled      bool
		expectedBase string
		expectedModel string
	}{
		{
			name:         "success - disabled without keys",
			cfg:          &config.Config{},
			enabled:      false,
			expectedBase: "",
			expectedModel: "gpt-5-mini",
		},
		{
			name:         "success - enabled via text key",
			cfg:          &config.Config{AITextGenerationAPIKey: "k", AITextGenerationBaseURL: "https://x", AITextGenerationModel: "m"},
			enabled:      true,
			expectedBase: "https://x",
			expectedModel: "m",
		},
		{
			name:         "success - dedicated overrides shared",
			cfg:          &config.Config{AITextGenerationAPIKey: "k", AITextGenerationBaseURL: "https://shared", AITextGenerationModel: "shared-m", AIThemGenAPIKey: "d", AIThemGenBaseURL: "https://ded", AIThemGenModel: "ded-m"},
			enabled:      true,
			expectedBase: "https://ded",
			expectedModel: "ded-m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.enabled, tt.cfg.IsThemeGenerationEnabled())
			assert.Equal(t, tt.expectedBase, tt.cfg.EffectiveThemGenBaseURL())
			assert.Equal(t, tt.expectedModel, tt.cfg.EffectiveThemGenModel())
		})
	}
}
