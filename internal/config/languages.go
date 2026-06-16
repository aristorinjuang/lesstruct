package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type languagesConfig struct {
	Languages []string `toml:"languages"`
}

// LoadLanguages loads the supported languages from the config.toml file.
// Returns at least ["en"] — if the config file is missing or has no languages
// defined, "en" is used as the primary language.
func LoadLanguages(cfg *Config) ([]string, error) {
	if cfg == nil {
		return []string{"en"}, nil
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"en"}, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", configPath, err)
	}

	var lc languagesConfig
	if err := toml.Unmarshal(data, &lc); err != nil {
		return nil, fmt.Errorf("parsing languages from %s: %w", configPath, err)
	}

	if len(lc.Languages) == 0 {
		return []string{"en"}, nil
	}

	return lc.Languages, nil
}

// PrimaryLanguage returns the first language in the slice, or "en" if empty.
func PrimaryLanguage(languages []string) string {
	if len(languages) == 0 {
		return "en"
	}
	return languages[0]
}
