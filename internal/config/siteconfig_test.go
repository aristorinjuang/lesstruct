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

func TestLoadSiteConfig_NameLogoAndFavicon(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `[site_config]
name = "Astra Motor Kalbar"
logo = "/uploads/logo.png"
favicon = "/uploads/icon.png"
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
	assert.Equal(t, "/uploads/icon.png", sc.Favicon)
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
	assert.Empty(t, sc.Favicon, "favicon should default to empty when unset")
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

func TestLoadSiteConfig_ProductImageRatio(t *testing.T) {
	tests := []struct {
		name        string
		configTOML  string
		expected    string
		wantErr     bool
		wantErrText string
	}{
		{
			name: "success - colon form normalizes with spaces",
			configTOML: `[site_config]
product_image_ratio = "1:1"
`,
			expected: "1 / 1",
			wantErr:  false,
		},
		{
			name: "success - slash form normalizes with spaces",
			configTOML: `[site_config]
product_image_ratio = "4/5"
`,
			expected: "4 / 5",
			wantErr:  false,
		},
		{
			name: "success - unset keeps empty for hard-coded default",
			configTOML: `[site_config]
name = "My Site"
`,
			expected: "",
			wantErr:  false,
		},
		{
			name: "error - invalid shape names the field",
			configTOML: `[site_config]
product_image_ratio = "wide"
`,
			wantErr:     true,
			wantErrText: "product_image_ratio",
		},
		{
			name: "error - zero height names the field",
			configTOML: `[site_config]
product_image_ratio = "4:0"
`,
			wantErr:     true,
			wantErrText: "product_image_ratio",
		},
		{
			name: "error - NaN ratio is rejected",
			configTOML: `[site_config]
product_image_ratio = "NaN:1"
`,
			wantErr:     true,
			wantErrText: "product_image_ratio",
		},
		{
			name: "error - infinite ratio is rejected",
			configTOML: `[site_config]
product_image_ratio = "1:Inf"
`,
			wantErr:     true,
			wantErrText: "product_image_ratio",
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

			sc, err := config.LoadSiteConfig(cfg)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, sc.ProductImageRatio)
		})
	}
}

func TestLoadSiteConfig_HeroFields(t *testing.T) {
	tests := []struct {
		name         string
		configTOML   string
		wantImage    string
		wantTitle    string
		wantSubtitle string
		wantErr      bool
	}{
		{
			name: "success - unset keeps empty for bilingual default",
			configTOML: `[site_config]
name = "My Site"
`,
			wantImage:    "",
			wantTitle:    "",
			wantSubtitle: "",
			wantErr:      false,
		},
		{
			name: "success - hero overrides trim spaces",
			configTOML: `[site_config]
hero_image = " /uploads/hero.webp "
hero_title = " Custom title "
hero_subtitle = " Custom subtitle "
`,
			wantImage:    "/uploads/hero.webp",
			wantTitle:    "Custom title",
			wantSubtitle: "Custom subtitle",
			wantErr:      false,
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

			sc, err := config.LoadSiteConfig(cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantImage, sc.HeroImage)
			assert.Equal(t, tt.wantTitle, sc.HeroTitle)
			assert.Equal(t, tt.wantSubtitle, sc.HeroSubtitle)
		})
	}
}
