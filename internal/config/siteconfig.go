package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type siteConfigFile struct {
	SiteConfig SiteConfig `toml:"site_config"`
}

// normalizeProductImageRatio validates a product_image_ratio value ("W:H" or
// "W/H" with positive numbers) and returns the canonical "W / H" form used in
// the inline aspect-ratio style. An empty input yields an empty output.
func normalizeProductImageRatio(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	sep := ""
	if strings.Contains(trimmed, ":") {
		sep = ":"
	} else if strings.Contains(trimmed, "/") {
		sep = "/"
	} else {
		return "", fmt.Errorf("invalid product_image_ratio %q: must be \"W:H\" or \"W/H\" with positive numbers", raw)
	}
	parts := strings.Split(trimmed, sep)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid product_image_ratio %q: must be \"W:H\" or \"W/H\" with positive numbers", raw)
	}
	widthStr := strings.TrimSpace(parts[0])
	heightStr := strings.TrimSpace(parts[1])
	width, errWidth := strconv.ParseFloat(widthStr, 64)
	height, errHeight := strconv.ParseFloat(heightStr, 64)
	if errWidth != nil || errHeight != nil || width <= 0 || height <= 0 || math.IsNaN(width) || math.IsNaN(height) || math.IsInf(width, 0) || math.IsInf(height, 0) {
		return "", fmt.Errorf("invalid product_image_ratio %q: must be \"W:H\" or \"W/H\" with positive numbers", raw)
	}
	return widthStr + " / " + heightStr, nil
}

// SiteConfig is the optional [site_config] block from config.toml. It carries
// the site's text and visual identity — the values that appear on every public
// page and that the handler otherwise bakes into the binary (notably the
// PageTitle suffix and the og:site_name). All fields are optional; an absent
// block yields a zero-value SiteConfig, and the caller is expected to default
// Name to the application name when empty.
type SiteConfig struct {
	Name              string `toml:"name"`
	Logo              string `toml:"logo"`
	Favicon           string `toml:"favicon"`
	ProductImageRatio string `toml:"product_image_ratio"`
	HeroImage         string `toml:"hero_image"`
	HeroTitle         string `toml:"hero_title"`
	HeroSubtitle      string `toml:"hero_subtitle"`
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

	normalized, err := normalizeProductImageRatio(scf.SiteConfig.ProductImageRatio)
	if err != nil {
		return SiteConfig{}, err
	}
	scf.SiteConfig.ProductImageRatio = normalized
	scf.SiteConfig.HeroImage = strings.TrimSpace(scf.SiteConfig.HeroImage)
	scf.SiteConfig.HeroTitle = strings.TrimSpace(scf.SiteConfig.HeroTitle)
	scf.SiteConfig.HeroSubtitle = strings.TrimSpace(scf.SiteConfig.HeroSubtitle)

	return scf.SiteConfig, nil
}
