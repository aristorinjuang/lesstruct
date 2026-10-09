package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadModules_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	mc, err := config.LoadModules(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Empty(t, mc.Disabled, "absent block must disable nothing")
}

func TestLoadModules_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	mc, err := config.LoadModules(cfg)
	require.NoError(t, err, "missing config file should not error")
	assert.Empty(t, mc.Disabled, "absent block must disable nothing")
}

func TestLoadModules_Disabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[modules]\ndisabled = [\"comments\", \"textgen\"]\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	mc, err := config.LoadModules(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"comments", "textgen"}, mc.Disabled)
	assert.True(t, mc.IsDisabled("comments"))
	assert.False(t, mc.IsDisabled("auth"))
}

func TestLoadModules_ParseError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[modules\ndisabled = [\"comments\"]\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadModules(cfg)
	require.Error(t, err)
}

func TestLoadModules_NilConfig(t *testing.T) {
	_, err := config.LoadModules(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestLoadModules_PathTraversal(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "conf",
		ConfigFile: "../config.toml",
	}

	_, err := config.LoadModules(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}
