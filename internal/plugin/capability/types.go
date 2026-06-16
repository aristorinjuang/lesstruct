package capability

import (
	"fmt"
	"slices"
)

func validateDBPermission(perm string) error {
	switch perm {
	case "read:content", "read:media", "read:users",
		"write:content", "write:media", "write:users":
		return nil
	default:
		return fmt.Errorf("unknown database permission: %q", perm)
	}
}

// matchPattern performs simple pattern matching.
// Supports trailing "*" as wildcard (e.g., "https://api.example.com/*").
func matchPattern(pattern, url string) bool {
	if len(pattern) == 0 {
		return false
	}

	// Trailing "*" matches any prefix
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(url) >= len(prefix) && url[:len(prefix)] == prefix
	}

	return pattern == url
}

// Capabilities defines what a plugin is allowed to do.
// Mirrors the [capabilities] TOML section in a manifest file.
type Capabilities struct {
	HTTP      []string        `toml:"http"`
	Database  []string        `toml:"database"`
	RateLimit RateLimitConfig `toml:"rate_limit"`
}

// RateLimitConfig controls how often a plugin can call host functions.
type RateLimitConfig struct {
	RequestsPerMinute int `toml:"requests_per_minute"`
}

// Manifest represents a plugin's capability declaration.
// Placed alongside the .wasm file as <name>.manifest.
type Manifest struct {
	Name         string       `toml:"name"`
	Version      string       `toml:"version"`
	MaxMemoryMB  int          `toml:"max_memory_mb"`
	Capabilities Capabilities `toml:"capabilities"`
}

// HasHTTP reports whether the manifest declares any HTTP URL patterns.
func (m Manifest) HasHTTP() bool {
	return len(m.Capabilities.HTTP) > 0
}

// HasDatabase reports whether the manifest declares any database permissions.
func (m Manifest) HasDatabase() bool {
	return len(m.Capabilities.Database) > 0
}

// Validate checks that the manifest has required fields and valid permissions.
func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest version is required")
	}

	for _, perm := range m.Capabilities.Database {
		if err := validateDBPermission(perm); err != nil {
			return err
		}
	}

	if m.MaxMemoryMB < 0 {
		return fmt.Errorf("max_memory_mb must be non-negative")
	}

	return nil
}

// IsHTTPURLAllowed checks whether url is covered by any declared HTTP pattern.
// Uses simple prefix matching.
func (m Manifest) IsHTTPURLAllowed(url string) bool {
	for _, pattern := range m.Capabilities.HTTP {
		if matchPattern(pattern, url) {
			return true
		}
	}
	return false
}

// HasDBPermission checks whether the manifest allows a specific db permission.
func (m Manifest) HasDBPermission(perm string) bool {
	return slices.Contains(m.Capabilities.Database, perm)
}
