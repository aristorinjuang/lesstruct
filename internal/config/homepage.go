package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// HomepageSection is one [[homepage_section]] block from config.toml. It tells
// the public homepage to render a per-post-type grouping in addition to (or
// instead of relying on) the flat latest-posts list. PostType is the only
// required field; Limit and Title are optional overrides.
type HomepageSection struct {
	PostType string `toml:"post_type"`
	Limit    int    `toml:"limit"`
	Title    string `toml:"title"`
}

type homepageConfig struct {
	HomepageSections []HomepageSection `toml:"homepage_section"`
}

// LoadHomepageSections reads the [[homepage_section]] blocks from the same
// config.toml that supplies post types. It mirrors LoadPostTypes: a missing
// config directory or file yields an empty slice (the homepage then renders
// the flat latest-posts list — fully backward compatible).
func LoadHomepageSections(cfg *Config) ([]HomepageSection, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return nil, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — no sections configured.
		return nil, nil
	}

	var hc homepageConfig
	if err := toml.Unmarshal(data, &hc); err != nil {
		return nil, fmt.Errorf("failed to parse homepage sections config: %w", err)
	}

	sections := make([]HomepageSection, 0, len(hc.HomepageSections))
	for _, hs := range hc.HomepageSections {
		if strings.TrimSpace(hs.PostType) == "" {
			return nil, fmt.Errorf("homepage_section must have a post_type")
		}
		sections = append(sections, HomepageSection{
			PostType: hs.PostType,
			Limit:    hs.Limit,
			Title:    hs.Title,
		})
	}
	return sections, nil
}
