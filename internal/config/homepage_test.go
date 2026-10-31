package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadHomepageSections_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	sections, err := config.LoadHomepageSections(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Empty(t, sections, "expected no sections when config dir is missing")
}

func TestLoadHomepageSections_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	sections, err := config.LoadHomepageSections(cfg)
	require.NoError(t, err, "missing config file should not error")
	assert.Empty(t, sections, "expected no sections when config file is missing")
}

func TestLoadHomepageSections_NoSections(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`post_type = []`+"\n"), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	sections, err := config.LoadHomepageSections(cfg)
	require.NoError(t, err)
	assert.Empty(t, sections, "expected no sections when none are configured")
}

func TestLoadHomepageSections_WithSections(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `[[homepage_section]]
post_type = "article"
limit = 6
title = "Artikel Pilihan"

[[homepage_section]]
post_type = "article"
limit = 20
offset = 6
title = "Rekomendasi"

[[homepage_section]]
post_type = "event"
limit = 3
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	sections, err := config.LoadHomepageSections(cfg)
	require.NoError(t, err)
	require.Len(t, sections, 3)

	assert.Equal(t, "article", sections[0].PostType)
	assert.Equal(t, 6, sections[0].Limit)
	assert.Equal(t, 0, sections[0].Offset)
	assert.Equal(t, "Artikel Pilihan", sections[0].Title)

	assert.Equal(t, "article", sections[1].PostType)
	assert.Equal(t, 20, sections[1].Limit)
	assert.Equal(t, 6, sections[1].Offset)
	assert.Equal(t, "Rekomendasi", sections[1].Title)

	assert.Equal(t, "event", sections[2].PostType)
	assert.Equal(t, 3, sections[2].Limit)
	assert.Empty(t, sections[2].Title, "title should default to empty when unset")
}

func TestLoadHomepageSections_MissingPostType(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `[[homepage_section]]
limit = 6
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg := &config.Config{
		ConfigDir:  tempDir,
		ConfigFile: "config.toml",
	}

	_, err := config.LoadHomepageSections(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have a post_type")
}

func TestLoadHomepageSections_NilConfig(t *testing.T) {
	_, err := config.LoadHomepageSections(nil)
	require.Error(t, err)
}

func TestLoadHomepageSections_PathTraversalRejected(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "config",
		ConfigFile: "../../etc/passwd",
	}

	_, err := config.LoadHomepageSections(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

func TestLoadHomepageSections_Localized(t *testing.T) {
	tests := []struct {
		name        string
		configTOML  string
		wantTitle   string
		wantTitleID string
		wantDesc    string
		wantDescID  string
		wantErr     bool
	}{
		{
			name: "success - localized title and description load",
			configTOML: `[[homepage_section]]
post_type = "article"
title = "Featured"
title_id = "Unggulan"
description = "Base desc"
description_id = "Deskripsi ID"
`,
			wantTitle:   "Featured",
			wantTitleID: "Unggulan",
			wantDesc:    "Base desc",
			wantDescID:  "Deskripsi ID",
			wantErr:     false,
		},
		{
			name: "success - missing localized falls back to empty",
			configTOML: `[[homepage_section]]
post_type = "article"
title = "Featured"
`,
			wantTitle:   "Featured",
			wantTitleID: "",
			wantDesc:    "",
			wantDescID:  "",
			wantErr:     false,
		},
		{
			name: "error - title_id too long",
			configTOML: `[[homepage_section]]
post_type = "article"
title_id = "` + strings.Repeat("a", 201) + `"
`,
			wantErr: true,
		},
		{
			name: "error - description too long",
			configTOML: `[[homepage_section]]
post_type = "article"
description = "` + strings.Repeat("a", 501) + `"
`,
			wantErr: true,
		},
		{
			name: "error - description_id too long",
			configTOML: `[[homepage_section]]
post_type = "article"
description_id = "` + strings.Repeat("a", 501) + `"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.toml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.configTOML), 0644))

			cfg := &config.Config{
				ConfigDir:  tempDir,
				ConfigFile: "config.toml",
			}

			sections, err := config.LoadHomepageSections(cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, sections, 1)
			assert.Equal(t, tt.wantTitle, sections[0].Title)
			assert.Equal(t, tt.wantTitleID, sections[0].TitleID)
			assert.Equal(t, tt.wantDesc, sections[0].Description)
			assert.Equal(t, tt.wantDescID, sections[0].DescriptionID)
		})
	}
}
