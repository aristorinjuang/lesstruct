package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type catalogConfigFile struct {
	Catalog catalogBlockFile `toml:"catalog"`
}

type catalogBlockFile struct {
	PostType          string            `toml:"post_type"`
	Currency          string            `toml:"currency"`
	NotifyEmail       string            `toml:"notify_email"`
	DisplayCurrency   []DisplayCurrency `toml:"display_currency"`
	DisplayCurrencies []DisplayCurrency `toml:"display_currencies"`
}

// isUpperAlpha reports whether every rune is A-Z.
func isUpperAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// CatalogConfig is the optional [catalog] block from config.toml: which
// registered post type carries products, the display currency for public
// payloads, the display-only currencies for the storefront toggle, and the
// low-stock notice recipient. Loaded by the composition root and mapped into
// the catalog module's options.

type CatalogConfig struct {
	PostType          string            `toml:"post_type"`
	Currency          string            `toml:"currency"`
	NotifyEmail       string            `toml:"notify_email"`
	DisplayCurrencies []DisplayCurrency `toml:"display_currency"`
}

// DisplayCurrency is one display-only currency for the storefront toggle:
// the ISO code the visitor picks and the conversion rate (display-major per
// base-minor, applied with cents scaling for two-decimal display currencies
// in storefront.js). Checkout stays in the base currency.
type DisplayCurrency struct {
	Code string  `toml:"code"`
	Rate float64 `toml:"rate"`
}

// LoadCatalog reads the optional [catalog] block from the same config.toml
// that supplies post types and the [modules] list. An absent block or file
// yields defaults ("product" / "IDR") — backward compatible, and the
// commerce edition works out of the box with the documented config.
func LoadCatalog(cfg *Config) (CatalogConfig, error) {
	if cfg == nil {
		return CatalogConfig{}, fmt.Errorf("config cannot be nil")
	}

	config := CatalogConfig{PostType: "product", Currency: "IDR"}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return CatalogConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return CatalogConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — defaults apply.
		return config, nil
	}

	var file catalogConfigFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return CatalogConfig{}, fmt.Errorf("failed to parse catalog config: %w", err)
	}

	if file.Catalog.PostType != "" {
		config.PostType = strings.TrimSpace(file.Catalog.PostType)
	}
	if file.Catalog.Currency != "" {
		config.Currency = strings.TrimSpace(strings.ToUpper(file.Catalog.Currency))
	}
	// An unset address disables the notice: the adapter degrades to
	// silence and the domain never knows.
	config.NotifyEmail = strings.TrimSpace(file.Catalog.NotifyEmail)

	if len(config.Currency) != 3 || !isUpperAlpha(config.Currency) {
		return CatalogConfig{}, fmt.Errorf("[catalog] currency must be a 3-letter uppercase code (got %q)", config.Currency)
	}

	displays := append([]DisplayCurrency{}, file.Catalog.DisplayCurrency...)
	displays = append(displays, file.Catalog.DisplayCurrencies...)
	seen := make(map[string]bool, len(displays))
	normalized := make([]DisplayCurrency, 0, len(displays))
	for i := range displays {
		code := strings.TrimSpace(strings.ToUpper(displays[i].Code))
		rate := displays[i].Rate
		if len(code) != 3 || !isUpperAlpha(code) {
			return CatalogConfig{}, fmt.Errorf("[catalog] display_currency[%d].code must be a 3-letter uppercase code (got %q)", i, displays[i].Code)
		}
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return CatalogConfig{}, fmt.Errorf("[catalog] display_currency[%d].rate must be greater than zero (got %v)", i, rate)
		}
		if code == config.Currency {
			return CatalogConfig{}, fmt.Errorf("[catalog] display_currency[%d].code must differ from the base currency %q", i, config.Currency)
		}
		if seen[code] {
			return CatalogConfig{}, fmt.Errorf("[catalog] display_currency[%d].code duplicates %q", i, code)
		}
		seen[code] = true
		normalized = append(normalized, DisplayCurrency{Code: code, Rate: rate})
	}
	if len(normalized) > 0 {
		config.DisplayCurrencies = normalized
	}

	return config, nil
}
