package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type mcpConfigFile struct {
	MCP MCPConfig `toml:"mcp"`
}

// MCPConfig is the optional [mcp] block from config.toml. When Enabled is
// true, the server mounts the Streamable HTTP MCP endpoint at POST /mcp, so a
// remote host can drive the /api/v1 surface with a Bearer API key without a
// local binary. An absent block yields a zero-value MCPConfig (disabled — the
// new network surface is opt-in).
type MCPConfig struct {
	Enabled bool `toml:"enabled"`
}

// LoadMCP reads the optional [mcp] block from the same config.toml that
// supplies post types and homepage sections. It mirrors LoadHeadless: a
// missing config directory or file yields a zero-value MCPConfig (fully
// backward compatible).
func LoadMCP(cfg *Config) (MCPConfig, error) {
	if cfg == nil {
		return MCPConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return MCPConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return MCPConfig{}, nil
		}
		return MCPConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — the MCP endpoint stays disabled.
		return MCPConfig{}, nil
	}

	var mcf mcpConfigFile
	if err := toml.Unmarshal(data, &mcf); err != nil {
		return MCPConfig{}, fmt.Errorf("failed to parse mcp config: %w", err)
	}

	return mcf.MCP, nil
}
