package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type invoiceConfigFile struct {
	Invoice struct {
		SellerName    string   `toml:"seller_name"`
		SellerAddress string   `toml:"seller_address"`
		SellerTaxID   string   `toml:"seller_tax_id"`
		TaxRate       *float64 `toml:"tax_rate"`
	} `toml:"invoice"`
}

// invoiceWriteFile is the block SaveInvoice renders back into config.toml:
// every key is always written, so an explicit zero rate survives.
type invoiceWriteFile struct {
	Invoice InvoiceConfig `toml:"invoice"`
}

// marshalInvoiceBlock renders the [invoice] table. BurntSushi owns the TOML
// escaping, so quotes, backslashes, and newlines in the seller identity stay
// valid no matter what the admin types.
func marshalInvoiceBlock(settings InvoiceConfig) (string, error) {
	data, err := toml.Marshal(invoiceWriteFile{Invoice: settings})
	if err != nil {
		return "", fmt.Errorf("failed to render invoice config: %w", err)
	}
	return string(data), nil
}

// isInvoiceHeader reports whether the line opens the [invoice] table,
// tolerating surrounding whitespace and a trailing comment. Sub-tables such
// as [invoice.extra] do not match.
func isInvoiceHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return false
	}
	closing := strings.IndexByte(trimmed, ']')
	if closing < 0 {
		return false
	}
	return strings.TrimSpace(trimmed[1:closing]) == "invoice"
}

// replaceInvoiceBlock swaps the [invoice] table for block and leaves every
// other line untouched. The block ends at the next table header; without one
// it runs to the end of the file. A file without the table gains it at the
// end, separated by one blank line.
func replaceInvoiceBlock(content, block string) string {
	lines := strings.SplitAfter(content, "\n")
	start := -1
	for index, line := range lines {
		if isInvoiceHeader(line) {
			start = index
			break
		}
	}
	if start == -1 {
		if content != "" {
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			if !strings.HasSuffix(content, "\n\n") {
				content += "\n"
			}
		}
		return content + block
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "[") {
			end = index
			break
		}
	}

	var updated strings.Builder
	updated.WriteString(strings.Join(lines[:start], ""))
	updated.WriteString(block)
	rest := strings.Join(lines[end:], "")
	if rest != "" && !strings.HasPrefix(rest, "\n") {
		updated.WriteString("\n")
	}
	updated.WriteString(rest)
	return updated.String()
}

// InvoiceConfig is the optional [invoice] block from config.toml: the
// seller identity stamped on every invoice and the PPN rate applied as a
// display-only breakdown. Loaded by the composition root and mapped into
// the orders module's options.
type InvoiceConfig struct {
	SellerName    string  `toml:"seller_name"`
	SellerAddress string  `toml:"seller_address"`
	SellerTaxID   string  `toml:"seller_tax_id"`
	TaxRate       float64 `toml:"tax_rate"`
}

// LoadInvoice reads the optional [invoice] block from the same config.toml
// that supplies post types and the [modules] list. An absent block or file
// yields blank seller identity with the statutory PPN default —
// backward compatible, and invoices render out of the box.
func LoadInvoice(cfg *Config) (InvoiceConfig, error) {
	if cfg == nil {
		return InvoiceConfig{}, fmt.Errorf("config cannot be nil")
	}

	config := InvoiceConfig{TaxRate: 12}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return InvoiceConfig{}, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return InvoiceConfig{}, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is not an error — defaults apply.
		return config, nil
	}

	var file invoiceConfigFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return InvoiceConfig{}, fmt.Errorf("failed to parse invoice config: %w", err)
	}

	config.SellerName = strings.TrimSpace(file.Invoice.SellerName)
	config.SellerAddress = strings.TrimSpace(file.Invoice.SellerAddress)
	config.SellerTaxID = strings.TrimSpace(file.Invoice.SellerTaxID)
	// The rate rides a pointer so an explicit zero (a non-PKP seller
	// charging no PPN) survives: only an absent key takes the default.
	if file.Invoice.TaxRate != nil {
		config.TaxRate = *file.Invoice.TaxRate
	}
	if config.TaxRate < 0 || config.TaxRate > 100 {
		return InvoiceConfig{}, fmt.Errorf("failed to parse invoice config: tax_rate %v is not a percent", config.TaxRate)
	}

	return config, nil
}

// SaveInvoice rewrites only the [invoice] block of config.toml, preserving
// every other line verbatim. The block itself is regenerated, so comments
// inside it do not survive; comments elsewhere in the file stay intact. The
// write is atomic — a temp file in the same directory renames over the
// original — and an explicit zero rate is written back as zero.
func SaveInvoice(cfg *Config, settings InvoiceConfig) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}
	if settings.TaxRate < 0 || settings.TaxRate > 100 {
		return fmt.Errorf("tax_rate %v is not a percent", settings.TaxRate)
	}

	settings.SellerName = strings.TrimSpace(settings.SellerName)
	settings.SellerAddress = strings.TrimSpace(settings.SellerAddress)
	settings.SellerTaxID = strings.TrimSpace(settings.SellerTaxID)

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	block, err := marshalInvoiceBlock(settings)
	if err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(configPath); statErr == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(cfg.ConfigDir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.WriteString(replaceInvoiceBlock(string(data), block)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temporary config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary config file: %w", err)
	}
	if err := os.Rename(tmpName, configPath); err != nil {
		return fmt.Errorf("failed to replace config file %s: %w", configPath, err)
	}
	return nil
}
