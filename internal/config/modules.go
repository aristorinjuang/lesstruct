package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type modulesConfigFile struct {
	Modules ModulesConfig `toml:"modules"`
}

// ModulesConfig is the optional [modules] block from config.toml. Disabled
// names modules the composition root must not register: kernel.Prune drops
// them plus every transitive Requires dependent (cascade) before InitAll.
// Unknown names are warned about and ignored so configs stay portable across
// editions. An absent block yields a zero-value ModulesConfig (nothing
// disabled — the backward-compatible default). Headless mode is NOT
// expressed here: [headless] only suppresses public-surface mounting while
// the site-ssr engine stays registered for SSG/export.
type ModulesConfig struct {
	Disabled []string `toml:"disabled"`
}

// IsDisabled reports whether the named module is disabled.
func (c ModulesConfig) IsDisabled(name string) bool {
	return slices.Contains(c.Disabled, name)
}

// LoadModules reads the optional [modules] block from the same config.toml
// that supplies post types and homepage sections. It mirrors LoadHeadless: a
// missing config directory or file yields a zero-value ModulesConfig (fully
// backward compatible).
func LoadModules(cfg *Config) (ModulesConfig, error) {
	if cfg == nil {
		return ModulesConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return ModulesConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return ModulesConfig{}, nil
		}
		return ModulesConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — no modules disabled.
		return ModulesConfig{}, nil
	}

	var mcf modulesConfigFile
	if err := toml.Unmarshal(data, &mcf); err != nil {
		return ModulesConfig{}, fmt.Errorf("failed to parse modules config: %w", err)
	}

	return mcf.Modules, nil
}
