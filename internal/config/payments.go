package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type paymentsConfigFile struct {
	Payments PaymentsConfig `toml:"payments"`
}

// ManualPayee is the optional [payments.manual] block from config.toml:
// the destination account shown on the confirmation email ("How to pay")
// and the guest tracking panel for manual bank transfers. All-or-nothing:
// either all three fields are set or none is — a partial block fails
// startup naming the missing field, so operators never ship a payee panel
// missing the account number.
type ManualPayee struct {
	Bank    string `toml:"bank"`
	Account string `toml:"account"`
	Name    string `toml:"name"`
}

// PaymentsConfig is the optional [payments] block from config.toml: the
// offered payment methods, in the order the client presents them, plus
// the Midtrans gateway credentials read from the environment (secrets
// never ride config.toml). Loaded by the composition root and mapped
// into the payments module's options.
type PaymentsConfig struct {
	Methods              []string    `toml:"methods"`
	Manual               ManualPayee `toml:"manual"`
	MidtransServerKey    string
	MidtransIsProduction bool
}

// LoadPayments reads the optional [payments] block from the same
// config.toml that supplies post types and the [modules] list. An absent
// block or file yields the default method list — manual transfer first,
// so the commerce edition works out of the box with the documented
// config. Unknown values ride through: the composition root drops any
// method its drivers do not know, so newer configs can pre-list gateways
// before their drivers exist.
func LoadPayments(cfg *Config) (PaymentsConfig, error) {
	if cfg == nil {
		return PaymentsConfig{}, fmt.Errorf("config cannot be nil")
	}

	config := PaymentsConfig{Methods: []string{"manual"}}

	// The Midtrans credentials ride the environment: secrets belong to
	// .env, never config.toml. An unset server key leaves the gateway
	// unadopted (manual proofs remain). These read before the file so
	// every path — absent dir, absent file, parsed file — answers them.
	config.MidtransServerKey = os.Getenv("MIDTRANS_SERVER_KEY")
	config.MidtransIsProduction = strings.EqualFold(os.Getenv("MIDTRANS_IS_PRODUCTION"), "true")

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return PaymentsConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return PaymentsConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — defaults apply.
		return config, nil
	}

	var file paymentsConfigFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return PaymentsConfig{}, fmt.Errorf("failed to parse payments config: %w", err)
	}

	trimmed := make([]string, 0, len(file.Payments.Methods))
	for _, method := range file.Payments.Methods {
		method = strings.ToLower(strings.TrimSpace(method))
		if method == "" {
			continue
		}
		trimmed = append(trimmed, method)
	}
	if len(trimmed) > 0 {
		config.Methods = trimmed
	}

	manual := ManualPayee{
		Bank:    strings.TrimSpace(file.Payments.Manual.Bank),
		Account: strings.TrimSpace(file.Payments.Manual.Account),
		Name:    strings.TrimSpace(file.Payments.Manual.Name),
	}
	set := 0
	if manual.Bank != "" {
		set++
	}
	if manual.Account != "" {
		set++
	}
	if manual.Name != "" {
		set++
	}
	if set > 0 && set < 3 {
		missing := make([]string, 0, 2)
		if manual.Bank == "" {
			missing = append(missing, "bank")
		}
		if manual.Account == "" {
			missing = append(missing, "account")
		}
		if manual.Name == "" {
			missing = append(missing, "name")
		}
		return PaymentsConfig{}, fmt.Errorf("incomplete [payments.manual] block: missing %s (all of bank, account, name are required)", strings.Join(missing, ", "))
	}
	config.Manual = manual

	return config, nil
}
