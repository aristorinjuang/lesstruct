package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type headlessConfigFile struct {
	Headless HeadlessConfig `toml:"headless"`
}

// HeadlessConfig is the optional [headless] block from config.toml. When
// Enabled is true, the server-rendered content site (the /* catch-all) is not
// mounted — the instance serves only the admin panel and the REST API, for
// headless CMS use. An absent block yields a zero-value HeadlessConfig
// (Enabled=false — the content site renders as before).
type HeadlessConfig struct {
	Enabled bool `toml:"enabled"`
}

// LoadHeadless reads the optional [headless] block from the same config.toml
// that supplies post types and homepage sections. It mirrors
// LoadHomepageSections: a missing config directory or file yields a zero-value
// HeadlessConfig (fully backward compatible).
func LoadHeadless(cfg *Config) (HeadlessConfig, error) {
	if cfg == nil {
		return HeadlessConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return HeadlessConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return HeadlessConfig{}, nil
		}
		return HeadlessConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — headless mode not configured.
		return HeadlessConfig{}, nil
	}

	var hcf headlessConfigFile
	if err := toml.Unmarshal(data, &hcf); err != nil {
		return HeadlessConfig{}, fmt.Errorf("failed to parse headless config: %w", err)
	}

	return hcf.Headless, nil
}
