package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRolesConfig = `[[role]]
name = "Journalist"
post_types = ["article"]
publish = false
media = true
comments = true

[[role]]
name = "Editor"
post_types = ["article", "event"]
publish = true
`

func writeTestConfig(t *testing.T, content string) *config.Config {
	t.Helper()
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.Mkdir(configDir, 0755), "Failed to create config dir")
	configPath := filepath.Join(configDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644), "Failed to write config")
	return &config.Config{
		ConfigDir:  configDir,
		ConfigFile: "config.toml",
	}
}

func TestLoadRoles_MissingConfigDir(t *testing.T) {
	cfg := &config.Config{ConfigDir: "nonexistent-config-dir"}

	service, err := config.LoadRoles(cfg, []string{"post", "page"})
	require.NoError(t, err)
	require.NotNil(t, service)

	// Only the built-in roles survive when no config exists.
	assert.Len(t, service.GetAll(), 3)
}

func TestLoadRoles_WithCustomRoles(t *testing.T) {
	cfg := writeTestConfig(t, testRolesConfig)

	service, err := config.LoadRoles(cfg, []string{"post", "page", "article", "event"})
	require.NoError(t, err)
	require.NotNil(t, service)

	// 3 built-ins + 2 custom.
	assert.Len(t, service.GetAll(), 5)

	assert.True(t, service.IsAssignable("Journalist"))
	assert.True(t, service.CanManageType("Journalist", "article"))
	assert.False(t, service.CanManageType("Journalist", "event"))
	assert.False(t, service.CanPublish("Journalist"))
	assert.True(t, service.CanMedia("Journalist"))
	assert.True(t, service.CanComment("Journalist"))

	assert.True(t, service.CanManageType("Editor", "article"))
	assert.True(t, service.CanManageType("Editor", "event"))
	assert.True(t, service.CanPublish("Editor"))
}

func TestLoadRoles_UnknownPostType(t *testing.T) {
	cfg := writeTestConfig(t, testRolesConfig)

	// "article" is not in the registered post types → fail closed.
	_, err := config.LoadRoles(cfg, []string{"post", "page"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown post type")
}

func TestLoadRoles_AdminReserved(t *testing.T) {
	cfg := writeTestConfig(t, `[[role]]
name = "Admin"
publish = false
`)

	_, err := config.LoadRoles(cfg, []string{"post"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestLoadRoles_NilConfig(t *testing.T) {
	_, err := config.LoadRoles(nil, []string{"post"})
	require.Error(t, err)
}

func TestLoadRegistration_MissingConfigDir(t *testing.T) {
	cfg := &config.Config{ConfigDir: "nonexistent-config-dir"}

	reg, err := config.LoadRegistration(cfg)
	require.NoError(t, err)
	// Absent block keeps the legacy coupling: enabled iff comments are enabled.
	assert.True(t, reg.IsEnabled(true))
	assert.False(t, reg.IsEnabled(false))
	assert.False(t, reg.AdminApproval)
	assert.Empty(t, reg.DefaultRole)
}

func TestLoadRegistration_WithBlock(t *testing.T) {
	cfg := writeTestConfig(t, `[registration]
enabled = true
default_role = "Journalist"
admin_approval = false
`)

	reg, err := config.LoadRegistration(cfg)
	require.NoError(t, err)
	assert.True(t, reg.IsEnabled(false), "explicit enabled=true overrides the comments fallback")
	assert.Equal(t, "Journalist", reg.DefaultRole)
	assert.False(t, reg.AdminApproval)
}

func TestLoadRegistration_AdminApproval(t *testing.T) {
	cfg := writeTestConfig(t, `[registration]
enabled = true
default_role = "Journalist"
admin_approval = true
`)

	reg, err := config.LoadRegistration(cfg)
	require.NoError(t, err)
	assert.True(t, reg.AdminApproval)
}

func TestLoadRegistration_ExplicitDisabled(t *testing.T) {
	cfg := writeTestConfig(t, `[registration]
enabled = false
`)

	reg, err := config.LoadRegistration(cfg)
	require.NoError(t, err)
	assert.False(t, reg.IsEnabled(true), "explicit enabled=false wins even when comments are enabled")
}

func TestLoadRegistration_NilConfig(t *testing.T) {
	_, err := config.LoadRegistration(nil)
	require.Error(t, err)
}