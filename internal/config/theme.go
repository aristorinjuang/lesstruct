package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// defaultThemesDir is the filesystem root for installed themes when
	// THEMES_DIR is unset.
	defaultThemesDir = "themes"
)

// themeFile mirrors the [theme] table for TOML decoding.
type themeFile struct {
	Theme struct {
		Active string `toml:"active"`
	} `toml:"theme"`
}

// themeWriteFile renders the [theme] table back into config.toml.
type themeWriteFile struct {
	Theme ThemeConfig `toml:"theme"`
}

// marshalThemeBlock renders the [theme] table. BurntSushi owns the TOML
// escaping so names with dashes stay valid.
func marshalThemeBlock(settings ThemeConfig) (string, error) {
	data, err := toml.Marshal(themeWriteFile{Theme: settings})
	if err != nil {
		return "", fmt.Errorf("failed to render theme config: %w", err)
	}
	return string(data), nil
}

// isThemeHeader reports whether the line opens the [theme] table,
// tolerating surrounding whitespace and a trailing comment. Sub-tables
// such as [theme.extra] do not match.
func isThemeHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return false
	}
	closing := strings.IndexByte(trimmed, ']')
	if closing < 0 {
		return false
	}
	return strings.TrimSpace(trimmed[1:closing]) == "theme"
}

// replaceThemeBlock swaps the [theme] table for block and leaves every
// other line untouched. The block ends at the next table header; without one
// it runs to the end of the file. A file without the table gains it at the
// end, separated by one blank line.
func replaceThemeBlock(content, block string) string {
	lines := strings.SplitAfter(content, "\n")
	start := -1
	for index, line := range lines {
		if isThemeHeader(line) {
			start = index
			break
		}
	}
	if start == -1 {
		if content != "" {
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			if !strings.HasSuffix(content, "\n\n") {
				content += "\n"
			}
		}
		return content + block
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "[") {
			end = index
			break
		}
	}

	var updated strings.Builder
	updated.WriteString(strings.Join(lines[:start], ""))
	updated.WriteString(block)
	rest := strings.Join(lines[end:], "")
	if rest != "" && !strings.HasPrefix(rest, "\n") {
		updated.WriteString("\n")
	}
	updated.WriteString(rest)
	return updated.String()
}

// ThemeConfig is the optional [theme] block from config.toml: the installed
// theme the content site layers over the embedded default. An empty Active
// means the embedded theme renders alone (unless THEME_DIR overrides it).
type ThemeConfig struct {
	Active string `toml:"active"`
}

// LoadTheme reads the optional [theme] block from the same config.toml
// that supplies post types and the [modules] list. An absent block or file
// yields an empty Active — the embedded default renders.
func LoadTheme(cfg *Config) (ThemeConfig, error) {
	if cfg == nil {
		return ThemeConfig{}, fmt.Errorf("config cannot be nil")
	}
	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return ThemeConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}
	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return ThemeConfig{}, nil
		}
		return ThemeConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}
	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ThemeConfig{}, nil
	}
	var file themeFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return ThemeConfig{}, fmt.Errorf("failed to parse theme config: %w", err)
	}
	return ThemeConfig{Active: strings.TrimSpace(file.Theme.Active)}, nil
}

// SaveTheme rewrites only the [theme] table of config.toml, preserving
// every other line verbatim. The write is atomic — a temp file in the same
// directory renames over the original.
func SaveTheme(cfg *Config, active string) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}
	settings := ThemeConfig{Active: strings.TrimSpace(active)}
	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}
	block, err := marshalThemeBlock(settings)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(configPath); statErr == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(cfg.ConfigDir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.WriteString(replaceThemeBlock(string(data), block)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temporary config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary config file: %w", err)
	}
	if err := os.Rename(tmpName, configPath); err != nil {
		return fmt.Errorf("failed to replace config file %s: %w", configPath, err)
	}
	return nil
}

// ThemesRoot returns the configured themes directory, defaulting to
// "themes/" when THEMES_DIR is unset or blank.
func (c *Config) ThemesRoot() string {
	if strings.TrimSpace(c.ThemesDir) == "" {
		return defaultThemesDir
	}
	return c.ThemesDir
}
