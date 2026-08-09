package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadHeadless_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	hc, err := config.LoadHeadless(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.False(t, hc.Enabled, "absent block must default to disabled")
}

func TestLoadHeadless_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	hc, err := config.LoadHeadless(cfg)
	require.NoError(t, err, "missing config file should not error")
	assert.False(t, hc.Enabled)
}

func TestLoadHeadless_Enabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[headless]\nenabled = true\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	hc, err := config.LoadHeadless(cfg)
	require.NoError(t, err)
	assert.True(t, hc.Enabled)
}

func TestLoadHeadless_DisabledExplicitly(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[headless]\nenabled = false\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	hc, err := config.LoadHeadless(cfg)
	require.NoError(t, err)
	assert.False(t, hc.Enabled)
}

func TestLoadHeadless_ParseError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[headless\nenabled = true\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadHeadless(cfg)
	require.Error(t, err)
}

func TestLoadHeadless_NilConfig(t *testing.T) {
	_, err := config.LoadHeadless(nil)
	require.Error(t, err)
}

func TestLoadHeadless_PathTraversalRejected(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "config",
		ConfigFile: "../../etc/passwd",
	}

	_, err := config.LoadHeadless(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

func TestLoadComments_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	cc, err := config.LoadComments(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.True(t, cc.IsEnabled(), "absent block must default to enabled")
}

func TestLoadComments_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	cc, err := config.LoadComments(cfg)
	require.NoError(t, err, "missing config file should not error")
	assert.True(t, cc.IsEnabled(), "absent block must default to enabled")
}

func TestLoadComments_EnabledExplicitly(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[comments]\nenabled = true\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	cc, err := config.LoadComments(cfg)
	require.NoError(t, err)
	assert.True(t, cc.IsEnabled())
}

func TestLoadComments_Disabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[comments]\nenabled = false\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	cc, err := config.LoadComments(cfg)
	require.NoError(t, err)
	assert.False(t, cc.IsEnabled())
}

func TestLoadComments_ParseError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[comments\nenabled = false\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadComments(cfg)
	require.Error(t, err)
}

func TestLoadComments_NilConfig(t *testing.T) {
	_, err := config.LoadComments(nil)
	require.Error(t, err)
}

func TestLoadComments_PathTraversalRejected(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "config",
		ConfigFile: "../../etc/passwd",
	}

	_, err := config.LoadComments(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}
