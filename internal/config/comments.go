package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type commentsConfigFile struct {
	Comments CommentsConfig `toml:"comments"`
}

// CommentsConfig is the optional [comments] block from config.toml. When
// disabled, the comment system is hard-disabled: comment routes are not
// mounted, the admin UI hides all comment surfaces, new content defaults to
// allowComments=false, and self-registration (which creates Commentator users)
// is blocked. An absent block yields a zero-value CommentsConfig, which
// IsEnabled treats as enabled — the backward-compatible default.
type CommentsConfig struct {
	Enabled *bool `toml:"enabled"`
}

// IsEnabled reports whether comments are enabled. The zero value (absent
// block or absent key) means enabled, so existing deployments without a
// [comments] block keep their comment system.
func (c CommentsConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// LoadComments reads the optional [comments] block from the same config.toml
// that supplies post types and homepage sections. It mirrors
// LoadHomepageSections: a missing config directory or file yields a zero-value
// CommentsConfig (which IsEnabled reports as enabled — fully backward
// compatible).
func LoadComments(cfg *Config) (CommentsConfig, error) {
	if cfg == nil {
		return CommentsConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return CommentsConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return CommentsConfig{}, nil
		}
		return CommentsConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — comments stay enabled by default.
		return CommentsConfig{}, nil
	}

	var ccf commentsConfigFile
	if err := toml.Unmarshal(data, &ccf); err != nil {
		return CommentsConfig{}, fmt.Errorf("failed to parse comments config: %w", err)
	}

	return ccf.Comments, nil
}
