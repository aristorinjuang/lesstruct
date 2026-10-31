package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type ordersConfigFile struct {
	Orders OrdersConfig `toml:"orders"`
}

// OrdersConfig is the optional [orders] block from config.toml: how long
// a pending order stays payable before the sweeper expires it. The same
// value mirrors into the Midtrans Snap session expiry, so a hosted
// checkout cannot outlive its order.
type OrdersConfig struct {
	PendingExpiry string `toml:"pending_expiry"`
}

// PendingExpiryDuration renders the configured window as a duration.
func (o OrdersConfig) PendingExpiryDuration() (time.Duration, error) {
	return time.ParseDuration(o.PendingExpiry)
}

// LoadOrders reads the optional [orders] block from the same config.toml
// that supplies post types and the [modules] list. An absent block or
// file yields the default — 24 hours, backward compatible with every
// existing deployment.
func LoadOrders(cfg *Config) (OrdersConfig, error) {
	resolved := OrdersConfig{PendingExpiry: "24h"}
	if cfg == nil {
		return OrdersConfig{}, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return OrdersConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return resolved, nil
		}
		return OrdersConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — defaults apply.
		return resolved, nil
	}

	var file ordersConfigFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return OrdersConfig{}, fmt.Errorf("failed to parse orders config: %w", err)
	}
	if file.Orders.PendingExpiry != "" {
		resolved.PendingExpiry = file.Orders.PendingExpiry
	}
	if _, err := time.ParseDuration(resolved.PendingExpiry); err != nil {
		return OrdersConfig{}, fmt.Errorf("failed to parse orders config: pending_expiry %q is not a duration: %w", resolved.PendingExpiry, err)
	}

	return resolved, nil
}
