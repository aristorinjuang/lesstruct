package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/aristorinjuang/lesstruct/internal/domain/role"
)

type rolesConfigFile struct {
	Roles []role.Role `toml:"role"`
}

// LoadRoles loads custom [[role]] definitions from the same config.toml that
// supplies post types. The built-in roles (Admin, Contributor, Commentator) are
// always present; a same-name [[role]] entry overrides a built-in (except
// Admin). postTypes must list every registered post-type slug — custom entries
// may only reference existing types, so a typo fails closed at startup instead
// of silently granting a role an unregistered type. A missing config file or
// directory yields a service with only the built-in roles.
func LoadRoles(cfg *Config, postTypes []string) (*role.Service, error) {
	service := role.NewService()

	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return nil, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return service, nil
		}
		return nil, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return service, nil
		}
		return nil, fmt.Errorf("failed to read roles config: %w", err)
	}

	var rcf rolesConfigFile
	if err := toml.Unmarshal(data, &rcf); err != nil {
		return nil, fmt.Errorf("failed to parse roles config: %w", err)
	}

	known := make(map[string]bool, len(postTypes))
	for _, s := range postTypes {
		known[s] = true
	}

	for _, r := range rcf.Roles {
		for _, slug := range r.PostTypes {
			if !known[slug] {
				return nil, fmt.Errorf("role %q references unknown post type %q", r.Name, slug)
			}
		}
		if err := service.Register(r); err != nil {
			return nil, fmt.Errorf("failed to register role %q: %w", r.Name, err)
		}
	}

	return service, nil
}