package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// SiteConfig is the optional [site_config] block from config.toml. It carries
// the site's text and visual identity — the values that appear on every public
// page and that the handler otherwise bakes into the binary (notably the
// PageTitle suffix and the og:site_name). Both fields are optional; an absent
// block yields a zero-value SiteConfig, and the caller is expected to default
// Name to the application name when empty.
type SiteConfig struct {
	Name string `toml:"name"`
	Logo string `toml:"logo"`
}

type siteConfigFile struct {
	SiteConfig SiteConfig `toml:"site_config"`
}

// LoadSiteConfig reads the optional [site_config] block from the same
// config.toml that supplies post types and homepage sections. It mirrors
// LoadHomepageSections: a missing config directory or file yields a zero-value
// SiteConfig (fully backward compatible).
func LoadSiteConfig(cfg *Config) (SiteConfig, error) {
	if cfg == nil {
		return SiteConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return SiteConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return SiteConfig{}, nil
		}
		return SiteConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — no site config configured.
		return SiteConfig{}, nil
	}

	var scf siteConfigFile
	if err := toml.Unmarshal(data, &scf); err != nil {
		return SiteConfig{}, fmt.Errorf("failed to parse site config: %w", err)
	}

	return scf.SiteConfig, nil
}
