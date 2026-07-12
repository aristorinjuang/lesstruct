package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSiteConfig_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	sc, err := config.LoadSiteConfig(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Equal(t, config.SiteConfig{}, sc, "expected zero-value when config dir is missing")
}

func TestLoadSiteConfig_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	sc, err := config.LoadSiteConfig(cfg)
	require.NoError(t, err, "missing config file should not error")
	assert.Equal(t, config.SiteConfig{}, sc, "expected zero-value when config file is missing")
}

func TestLoadSiteConfig_NoBlock(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`languages = ["en"]`+"\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	sc, err := config.LoadSiteConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, config.SiteConfig{}, sc, "expected zero-value when no [site_config] block is present")
}

func TestLoadSiteConfig_NameAndLogo(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `[site_config]
name = "Astra Motor Kalbar"
logo = "/uploads/logo.png"
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	sc, err := config.LoadSiteConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, "Astra Motor Kalbar", sc.Name)
	assert.Equal(t, "/uploads/logo.png", sc.Logo)
}

func TestLoadSiteConfig_NameOnly(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `[site_config]
name = "My Site"
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	sc, err := config.LoadSiteConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, "My Site", sc.Name)
	assert.Empty(t, sc.Logo, "logo should default to empty when unset")
}

func TestLoadSiteConfig_NilConfig(t *testing.T) {
	_, err := config.LoadSiteConfig(nil)
	require.Error(t, err)
}

func TestLoadSiteConfig_PathTraversalRejected(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "config",
		ConfigFile: "../../etc/passwd",
	}

	_, err := config.LoadSiteConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}
