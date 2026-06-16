package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/domain/thumbnail"
)

// LoadThumbnails loads thumbnail configurations from the config file (config.toml).
// If the file does not exist, it returns a service with default configs.
func LoadThumbnails(cfg *Config) (*thumbnail.Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	service := thumbnail.NewService()

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)

	if strings.Contains(configPath, "..") {
		return nil, fmt.Errorf("config path must not contain parent directory references")
	}

	configDirInfo, err := os.Stat(cfg.ConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			return service, nil
		}
		return nil, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	if !configDirInfo.IsDir() {
		return nil, fmt.Errorf("config path %s is not a directory", cfg.ConfigDir)
	}

	if err := service.LoadConfigFromFile(configPath); err != nil {
		return nil, fmt.Errorf("failed to load thumbnail config: %w", err)
	}

	return service, nil
}
