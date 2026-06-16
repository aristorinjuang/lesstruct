package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPostTypes_WithDefaultConfig(t *testing.T) {
	cfg := &config.Config{
		ConfigDir: "nonexistent-config-dir",
	}

	service, err := config.LoadPostTypes(cfg)
	require.NoError(t, err, "LoadPostTypes() failed")
	require.NotNil(t, service, "LoadPostTypes() returned nil service")

	// Should have default post types
	allTypes := service.GetAll()
	assert.Equal(t, 4, len(allTypes), "LoadPostTypes() returned wrong number of types")
}

func TestLoadPostTypes_WithCustomConfig(t *testing.T) {
	// Create a temporary config directory
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.Mkdir(configDir, 0755), "Failed to create config dir")

	// Write a custom post types config
	configContent := `[[post_type]]
name = "Portfolio"
slug = "portfolio"
description = "Portfolio items"
supports = ["title", "content", "tags", "featured_image"]
`
	configPath := filepath.Join(configDir, "post-types.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644), "Failed to write config")

	cfg := &config.Config{
		ConfigDir:  configDir,
		ConfigFile: "post-types.toml",
	}

	service, err := config.LoadPostTypes(cfg)
	require.NoError(t, err, "LoadPostTypes() failed")

	// Should have 5 types (4 default + 1 custom)
	allTypes := service.GetAll()
	assert.Equal(t, 5, len(allTypes), "LoadPostTypes() returned wrong number of types")

	// Check that custom type is loaded
	_, err = service.GetBySlug("portfolio")
	assert.NoError(t, err, "GetBySlug(portfolio) failed")
}

func TestLoadPostTypes_NilConfig(t *testing.T) {
	_, err := config.LoadPostTypes(nil)
	assert.Error(t, err, "LoadPostTypes() with nil config expected error")
}

func TestLoadPostTypes_CustomConfigFileName(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.Mkdir(configDir, 0755), "Failed to create config dir")

	configContent := `[[post_type]]
name = "Product"
slug = "product"
description = "Products"
supports = ["title", "content"]
`
	configPath := filepath.Join(configDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644), "Failed to write config")

	cfg := &config.Config{
		ConfigDir:  configDir,
		ConfigFile: "config.toml",
	}

	service, err := config.LoadPostTypes(cfg)
	require.NoError(t, err, "LoadPostTypes() failed")

	allTypes := service.GetAll()
	assert.Equal(t, 5, len(allTypes), "LoadPostTypes() returned wrong number of types")

	_, err = service.GetBySlug("product")
	assert.NoError(t, err, "GetBySlug(product) failed")
}

func TestLoadPostTypes_PathTraversalRejected(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "config",
		ConfigFile: "../../etc/passwd",
	}

	_, err := config.LoadPostTypes(cfg)
	assert.Error(t, err, "LoadPostTypes() with path traversal expected error")
	assert.Contains(t, err.Error(), "path separators")
}
