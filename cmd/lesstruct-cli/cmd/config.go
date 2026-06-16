package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// defaultBaseURL is used when no base URL is resolved from flag/env/config, so a
// bare `lesstruct-cli content create` works against a local server.
const defaultBaseURL = "http://localhost:8080"

// configFile is the parsed ~/.config/lesstruct/config.toml. Only the [default]
// table is read.
type configFile struct {
	Default struct {
		APIKey  string `toml:"api_key"`
		BaseURL string `toml:"base_url"`
	} `toml:"default"`
}

// resolveCredentials applies the documented precedence for both the API key and
// the base URL: flag → environment variable → config file [default]. A missing
// key after all sources is an error (the CLI never sends an unauthenticated
// request); a missing base URL falls back to defaultBaseURL. The flag values are
// passed in (resolved by the caller from cobra) so this function is pure and
// isolated per ExecuteArgs call.
func resolveCredentials(flagKey, flagBaseURL string) (apiKey string, baseURL string, err error) {
	apiKey = strings.TrimSpace(firstNonEmpty(flagKey, os.Getenv("LESSTRUCT_API_KEY")))
	baseURL = strings.TrimSpace(firstNonEmpty(flagBaseURL, os.Getenv("LESSTRUCT_BASE_URL")))

	if apiKey == "" || baseURL == "" {
		cfg, cfgErr := loadConfigFile()
		if cfgErr != nil {
			return "", "", cfgErr
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(cfg.Default.APIKey)
		}
		if baseURL == "" {
			baseURL = strings.TrimSpace(cfg.Default.BaseURL)
		}
	}

	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if apiKey == "" {
		return "", "", fmt.Errorf(
			"lesstruct-cli: no API key found (set --api-key, LESSTRUCT_API_KEY, or [default] api_key in %s)",
			configPathHint(),
		)
	}
	return apiKey, baseURL, nil
}

// firstNonEmpty returns the first non-empty argument, or "" if all are empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// loadConfigFile reads and parses the CLI config file, if present. A missing
// file is not an error (the precedence chain falls through to the no-key error);
// a present-but-unreadable or invalid file is surfaced so the user can fix it
// rather than chasing a misleading "no API key" message.
func loadConfigFile() (configFile, error) {
	path, ok := configFilePath()
	if !ok {
		return configFile{}, nil
	}
	if _, err := os.Stat(path); err != nil {
		return configFile{}, nil
	}
	var cfg configFile
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return configFile{}, fmt.Errorf("lesstruct-cli: config file %q could not be parsed: %w", path, err)
	}
	return cfg, nil
}

// configFilePath resolves the config file location: $LESSTRUCT_CONFIG, else
// $XDG_CONFIG_HOME/lesstruct/config.toml, else $HOME/.config/lesstruct/config.toml.
// ok is false when no candidate path can be determined.
func configFilePath() (string, bool) {
	if p := os.Getenv("LESSTRUCT_CONFIG"); p != "" {
		return p, true
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "lesstruct", "config.toml"), true
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".config", "lesstruct", "config.toml"), true
	}
	return "", false
}

// configPathHint returns a human-readable hint at the default config location
// for the missing-key error message.
func configPathHint() string {
	if p, ok := configFilePath(); ok {
		return p
	}
	return "~/.config/lesstruct/config.toml"
}
