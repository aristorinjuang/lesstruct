package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMCP(t *testing.T) {
	tests := []struct {
		name        string
		cfg         func(dir string) *config.Config
		write       bool
		content     string
		wantEnabled bool
		wantErr     string
	}{
		{
			name: "error - nil config",
			cfg: func(string) *config.Config {
				return nil
			},
			wantErr: "config cannot be nil",
		},
		{
			name: "success - missing config dir defaults to disabled",
			cfg: func(string) *config.Config {
				return &config.Config{ConfigDir: "nonexistent-config-dir", ConfigFile: "config.toml"}
			},
			wantEnabled: false,
		},
		{
			name: "success - missing config file defaults to disabled",
			cfg: func(dir string) *config.Config {
				return &config.Config{ConfigDir: dir, ConfigFile: "config.toml"}
			},
			wantEnabled: false,
		},
		{
			name: "success - enabled",
			cfg: func(dir string) *config.Config {
				return &config.Config{ConfigDir: dir, ConfigFile: "config.toml"}
			},
			write:       true,
			content:     "[mcp]\nenabled = true\n",
			wantEnabled: true,
		},
		{
			name: "success - disabled explicitly",
			cfg: func(dir string) *config.Config {
				return &config.Config{ConfigDir: dir, ConfigFile: "config.toml"}
			},
			write:       true,
			content:     "[mcp]\nenabled = false\n",
			wantEnabled: false,
		},
		{
			name: "error - parse failure",
			cfg: func(dir string) *config.Config {
				return &config.Config{ConfigDir: dir, ConfigFile: "config.toml"}
			},
			write:   true,
			content: "[mcp\nenabled = true\n",
			wantErr: "failed to parse mcp config",
		},
		{
			name: "error - path traversal rejected",
			cfg: func(string) *config.Config {
				return &config.Config{ConfigDir: "config", ConfigFile: "../../etc/passwd"}
			},
			wantErr: "path separators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := tt.cfg(dir)
			if tt.write {
				require.NoError(t, os.WriteFile(filepath.Join(dir, cfg.ConfigFile), []byte(tt.content), 0o600))
			}

			mc, err := config.LoadMCP(cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEnabled, mc.Enabled)
		})
	}
}
