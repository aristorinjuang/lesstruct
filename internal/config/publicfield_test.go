package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPublicFields_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err, "missing config dir should not error")
	require.NotNil(t, registry)
	assert.False(t, registry.IsQueryable("user", "", "points", "sort"),
		"empty registry should reject every query")
}

func TestLoadPublicFields_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err, "missing config file should not error")
	require.NotNil(t, registry)
	assert.False(t, registry.IsQueryable("user", "", "points", "sort"))
}

func TestLoadPublicFields_NoBlock(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`languages = ["en"]`+"\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)
	require.NotNil(t, registry)
	assert.False(t, registry.IsQueryable("user", "", "points", "sort"))
}

func TestLoadPublicFields_UserSortAndFilter(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = "points"
operations = ["sort", "filter"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)

	assert.True(t, registry.IsQueryable("user", "", "points", "sort"))
	assert.True(t, registry.IsQueryable("user", "", "points", "filter"))
	assert.False(t, registry.IsQueryable("user", "", "points", "delete"),
		"operation not in the allowlist should be rejected")
	assert.False(t, registry.IsQueryable("user", "", "rating", "sort"),
		"different field should be rejected")
	assert.False(t, registry.IsQueryable("content", "article", "points", "sort"),
		"different resource should be rejected")
}

func TestLoadPublicFields_ContentScopedToPostType(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "content"
field      = "views"
post_type  = "article"
operations = ["sort"]

[[public_field]]
resource   = "content"
field      = "category"
post_type  = ""
operations = ["filter"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)

	assert.True(t, registry.IsQueryable("content", "article", "views", "sort"))
	assert.False(t, registry.IsQueryable("content", "page", "views", "sort"),
		"scoped post_type should reject other types")
	assert.False(t, registry.IsQueryable("content", "article", "views", "filter"),
		"operation not declared for this entry should be rejected")

	assert.True(t, registry.IsQueryable("content", "article", "category", "filter"))
	assert.True(t, registry.IsQueryable("content", "page", "category", "filter"),
		"empty post_type should match every type")
}

func TestLoadPublicFields_PostTypeIgnoredForUser(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	// post_type is silently dropped when resource = "user".
	configContent := `
[[public_field]]
resource   = "user"
field      = "points"
post_type  = "article"
operations = ["sort"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)
	assert.True(t, registry.IsQueryable("user", "", "points", "sort"),
		"user queries should ignore post_type entirely")
}

func TestLoadPublicFields_CaseInsensitive(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "USER"
field      = "Points"
operations = ["SORT", "Filter"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)

	assert.True(t, registry.IsQueryable("user", "", "Points", "sort"),
		"resource and operation are matched case-insensitively after normalisation")
	assert.True(t, registry.IsQueryable("user", "", "Points", "filter"))
}

func TestLoadPublicFields_DuplicateOperationsCollapsed(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = "points"
operations = ["sort", "sort", "filter", "filter"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)
	assert.True(t, registry.IsQueryable("user", "", "points", "sort"))
	assert.True(t, registry.IsQueryable("user", "", "points", "filter"))
}

func TestLoadPublicFields_InvalidResource(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "session"
field      = "points"
operations = ["sort"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadPublicFields(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource must be")
}

func TestLoadPublicFields_EmptyField(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = ""
operations = ["sort"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadPublicFields(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field must be a non-empty slug")
}

func TestLoadPublicFields_InvalidOperation(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = "points"
operations = ["sort", "delete"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadPublicFields(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operations entry must be")
}

func TestLoadPublicFields_EmptyOperations(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = "points"
operations = []
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadPublicFields(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operations must contain at least one")
}

func TestLoadPublicFields_NilConfig(t *testing.T) {
	_, err := config.LoadPublicFields(nil)
	require.Error(t, err)
}

func TestLoadPublicFields_PathTraversalRejected(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "config",
		ConfigFile: "../../etc/passwd",
	}

	_, err := config.LoadPublicFields(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

func TestLoadPublicFields_ExposeOperation(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = "tier_point"
operations = ["sort", "expose"]

[[public_field]]
resource   = "user"
field      = "current_point"
operations = ["expose"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)

	assert.True(t, registry.IsQueryable("user", "", "tier_point", "sort"))
	assert.True(t, registry.IsQueryable("user", "", "tier_point", "expose"))
	assert.False(t, registry.IsQueryable("user", "", "tier_point", "filter"),
		"filter not declared for this entry")

	exposed := registry.ExposedFields("user", "")
	assert.ElementsMatch(t, []string{"tier_point", "current_point"}, exposed,
		"both fields with expose should be returned")

	contentExposed := registry.ExposedFields("content", "")
	assert.Empty(t, contentExposed,
		"no content-resource entries have expose")
}

func TestPublicFieldRegistry_ExposedFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = "points"
operations = ["sort"]

[[public_field]]
resource   = "user"
field      = "tier_point"
operations = ["expose"]

[[public_field]]
resource   = "user"
field      = "stars"
operations = ["expose"]

[[public_field]]
resource   = "content"
field      = "views"
operations = ["sort"]

[[public_field]]
resource   = "content"
field      = "rating"
post_type  = "product"
operations = ["filter"]

[[public_field]]
resource   = "content"
field      = "category"
operations = ["filter"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)

	t.Run("nil registry returns nil", func(t *testing.T) {
		var nilReg *config.PublicFieldRegistry
		assert.Nil(t, nilReg.ExposedFields("user", ""))
	})

	t.Run("user resource returns exposed slugs", func(t *testing.T) {
		exposed := registry.ExposedFields("user", "")
		assert.ElementsMatch(t, []string{"tier_point", "stars"}, exposed,
			"only tier_point and stars have expose for user resource")
	})

	t.Run("content resource returns empty (expose not supported)", func(t *testing.T) {
		exposed := registry.ExposedFields("content", "")
		assert.Empty(t, exposed,
			"expose is not supported on content resource")
	})

	t.Run("unknown resource", func(t *testing.T) {
		exposed := registry.ExposedFields("session", "")
		assert.Empty(t, exposed)
	})
}

func TestPublicFieldRegistry_ExposedFields_Deduped(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "user"
field      = "tier_point"
operations = ["expose"]

[[public_field]]
resource   = "user"
field      = "tier_point"
operations = ["sort", "expose"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	registry, err := config.LoadPublicFields(cfg)
	require.NoError(t, err)

	exposed := registry.ExposedFields("user", "")
	assert.Equal(t, []string{"tier_point"}, exposed,
		"overlapping entries should not produce duplicate slugs")
}

func TestLoadPublicFields_ExposeOnContentRejected(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[[public_field]]
resource   = "content"
field      = "views"
operations = ["sort", "expose"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadPublicFields(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for resource")
}

func TestPublicFieldRegistry_NilRegistry(t *testing.T) {
	var registry *config.PublicFieldRegistry
	assert.False(t, registry.IsQueryable("user", "", "points", "sort"),
		"a nil registry should reject every query (fail-closed)")
	assert.False(t, registry.IsQueryable("user", "", "", "sort"),
		"empty field should be rejected regardless of registry state")
	assert.False(t, registry.IsQueryable("user", "", "points", ""),
		"empty operation should be rejected regardless of registry state")
}
