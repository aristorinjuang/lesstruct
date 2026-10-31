package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadInvoice_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	ic, err := config.LoadInvoice(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Equal(t, "", ic.SellerName, "defaults must apply")
	assert.Equal(t, float64(12), ic.TaxRate, "the statutory PPN default applies")
}

func TestLoadInvoice(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantName    string
		wantAddress string
		wantTaxID   string
		wantRate    float64
		wantErr     string
	}{
		{
			name:     "absent block yields defaults",
			content:  "[site_config]\nname = \"x\"\n",
			wantRate: 12,
		},
		{
			name:        "explicit values win",
			content:     "[invoice]\nseller_name = \"Lesstruct Demo\"\nseller_address = \"Jl. Example No. 1\"\nseller_tax_id = \"01.234.567.8-901.234\"\ntax_rate = 11\n",
			wantName:    "Lesstruct Demo",
			wantAddress: "Jl. Example No. 1",
			wantTaxID:   "01.234.567.8-901.234",
			wantRate:    11,
		},
		{
			name:     "an explicit zero rate survives for non-PKP sellers",
			content:  "[invoice]\ntax_rate = 0\n",
			wantRate: 0,
		},
		{
			name:    "a negative rate refuses",
			content: "[invoice]\ntax_rate = -3\n",
			wantErr: "tax_rate -3 is not a percent",
		},
		{
			name:    "an over-hundred rate refuses",
			content: "[invoice]\ntax_rate = 101\n",
			wantErr: "tax_rate 101 is not a percent",
		},
		{
			name:    "malformed toml refuses",
			content: "[invoice]\ntax_rate = \"twelve\"\n",
			wantErr: "failed to parse invoice config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writePaymentsConfig(t, tt.content)

			ic, err := config.LoadInvoice(cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, ic.SellerName)
			assert.Equal(t, tt.wantAddress, ic.SellerAddress)
			assert.Equal(t, tt.wantTaxID, ic.SellerTaxID)
			assert.Equal(t, tt.wantRate, ic.TaxRate)
		})
	}
}

func TestLoadInvoice_NilConfig(t *testing.T) {
	_, err := config.LoadInvoice(nil)
	require.Error(t, err)
}

func TestSaveInvoice(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		settings     config.InvoiceConfig
		wantName     string
		wantAddress  string
		wantTaxID    string
		wantRate     float64
		wantPreserve []string
		wantAbsent   []string
		wantErr      string
	}{
		{
			name:         "replaces an existing block and preserves the rest",
			content:      "# top comment\n[site_config]\nname = \"demo\" # trailing\n\n[invoice]\nseller_name = \"Old\"\ntax_rate = 12\n\n[modules]\nenabled = [\"blog\"]\n",
			settings:     config.InvoiceConfig{SellerName: "New Seller", SellerAddress: "Jl. Baru 2", SellerTaxID: "01.234", TaxRate: 11},
			wantName:     "New Seller",
			wantAddress:  "Jl. Baru 2",
			wantTaxID:    "01.234",
			wantRate:     11,
			wantPreserve: []string{"# top comment", "[site_config]", "name = \"demo\" # trailing", "[modules]", "enabled = [\"blog\"]"},
			wantAbsent:   []string{"seller_name = \"Old\""},
		},
		{
			name:         "appends the block when the file has none",
			content:      "[site_config]\nname = \"demo\"\n",
			settings:     config.InvoiceConfig{SellerName: "Seller", TaxRate: 0},
			wantName:     "Seller",
			wantRate:     0,
			wantPreserve: []string{"[site_config]", "name = \"demo\""},
		},
		{
			name:         "zero survives beside a following table",
			content:      "[invoice]\ntax_rate = 12\n\n[modules]\nenabled = []\n",
			settings:     config.InvoiceConfig{TaxRate: 0},
			wantRate:     0,
			wantPreserve: []string{"[modules]", "enabled = []"},
		},
		{
			name:        "quotes and newlines round-trip through TOML escaping",
			content:     "",
			settings:    config.InvoiceConfig{SellerName: "A \"B\" \\ C", SellerAddress: "Line one\nLine two", TaxRate: 12},
			wantName:    "A \"B\" \\ C",
			wantAddress: "Line one\nLine two",
			wantRate:    12,
		},
		{
			name:     "an over-hundred rate refuses",
			settings: config.InvoiceConfig{TaxRate: 101},
			wantErr:  "tax_rate 101 is not a percent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writePaymentsConfig(t, tt.content)

			err := config.SaveInvoice(cfg, tt.settings)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)

			loaded, err := config.LoadInvoice(cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, loaded.SellerName)
			assert.Equal(t, tt.wantAddress, loaded.SellerAddress)
			assert.Equal(t, tt.wantTaxID, loaded.SellerTaxID)
			assert.Equal(t, tt.wantRate, loaded.TaxRate)

			raw, err := os.ReadFile(filepath.Join(cfg.ConfigDir, cfg.ConfigFile))
			require.NoError(t, err)
			for _, snippet := range tt.wantPreserve {
				assert.Contains(t, string(raw), snippet)
			}
			for _, snippet := range tt.wantAbsent {
				assert.NotContains(t, string(raw), snippet)
			}
		})
	}
}

func TestSaveInvoice_MissingFileCreatesIt(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{ConfigDir: dir, ConfigFile: "config.toml"}

	err := config.SaveInvoice(cfg, config.InvoiceConfig{TaxRate: 0})

	require.NoError(t, err)
	loaded, err := config.LoadInvoice(cfg)
	require.NoError(t, err)
	assert.Equal(t, float64(0), loaded.TaxRate)
}

func TestSaveInvoice_NilConfig(t *testing.T) {
	err := config.SaveInvoice(nil, config.InvoiceConfig{})
	require.Error(t, err)
}
