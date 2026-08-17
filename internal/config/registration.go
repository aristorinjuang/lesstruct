package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// registrationConfigFile mirrors the TOML shape of the [registration] block.
// It keeps the parse shape separate from the public RegistrationConfig so
// unknown keys in config.toml are simply ignored by the TOML decoder.
type registrationConfigFile struct {
	Registration registrationConfigParse `toml:"registration"`
}

type registrationConfigParse struct {
	Enabled       *bool  `toml:"enabled"`
	DefaultRole   string `toml:"default_role"`
	AdminApproval bool   `toml:"admin_approval"`
}

// RegistrationConfig is the optional [registration] block from config.toml. It
// decouples self-registration from the comment system: today registration is
// enabled iff comments are enabled (because the only self-registerable role,
// Commentator, was meaningless without them). With custom [[role]] entries a
// site may want public registration for a non-comment role (e.g. a journalist
// role), so this block overrides the default coupling.
//
// Email verification is always mandatory: registrants are always created
// "pending" and must verify their email address. AdminApproval adds a second
// activation layer on top of the email proof.
//
// Absent block behavior (backward compatible):
//   - Enabled: follows the comment system (comments enabled ⇒ registration on).
//   - DefaultRole: "Commentator".
//   - AdminApproval: false (email verification activates the account).
type RegistrationConfig struct {
	Enabled       *bool
	DefaultRole   string
	AdminApproval bool
}

// IsEnabled reports whether self-registration is allowed. An explicit enabled
// key wins; otherwise it falls back to the comment system's state.
func (c RegistrationConfig) IsEnabled(commentsEnabled bool) bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	return commentsEnabled
}

// LoadRegistration reads the optional [registration] block from the same
// config.toml that supplies post types. It mirrors LoadComments: a missing
// config directory or file yields a zero-value RegistrationConfig, whose
// IsEnabled(commentsEnabled) reproduces the legacy coupling.
func LoadRegistration(cfg *Config) (RegistrationConfig, error) {
	if cfg == nil {
		return RegistrationConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return RegistrationConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return RegistrationConfig{}, nil
		}
		return RegistrationConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — registration keeps its legacy default.
		return RegistrationConfig{}, nil
	}

	var rcf registrationConfigFile
	if err := toml.Unmarshal(data, &rcf); err != nil {
		return RegistrationConfig{}, fmt.Errorf("failed to parse registration config: %w", err)
	}

	return RegistrationConfig{
		Enabled:       rcf.Registration.Enabled,
		DefaultRole:   rcf.Registration.DefaultRole,
		AdminApproval: rcf.Registration.AdminApproval,
	}, nil
}