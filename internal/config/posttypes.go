package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
)

// LoadPostTypes loads custom post types from the config directory
// If the config file doesn't exist, it returns a service with default post types
func LoadPostTypes(cfg *Config) (*posttype.Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return nil, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	service := posttype.NewService()

	// Validate config directory exists and is readable
	configDirInfo, err := os.Stat(cfg.ConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Config directory doesn't exist, use defaults
			return service, nil
		}
		return nil, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	if !configDirInfo.IsDir() {
		return nil, fmt.Errorf("config path %s is not a directory", cfg.ConfigDir)
	}

	// Check directory is readable by attempting to open it
	testFile, err := os.Open(cfg.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("config directory %s is not readable: %w", cfg.ConfigDir, err)
	}
	_ = testFile.Close()

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	if err := service.LoadConfigFromFile(configPath); err != nil {
		return nil, fmt.Errorf("failed to load post types config: %w", err)
	}

	return service, nil
}
