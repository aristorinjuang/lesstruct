package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o600))
	return &config.Config{ConfigDir: dir, ConfigFile: "config.toml"}
}

func TestLoadCatalog_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	cc, err := config.LoadCatalog(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Equal(t, "product", cc.PostType, "defaults must apply")
	assert.Equal(t, "IDR", cc.Currency, "defaults must apply")
}

func TestLoadCatalog_NoFile(t *testing.T) {
	cfg := &config.Config{ConfigDir: t.TempDir(), ConfigFile: "config.toml"}

	cc, err := config.LoadCatalog(cfg)
	require.NoError(t, err)
	assert.Equal(t, "product", cc.PostType)
	assert.Equal(t, "IDR", cc.Currency)
}

func TestLoadCatalog(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantType     string
		wantCurrency string
		wantNotify   string
		wantDisplays []config.DisplayCurrency
		wantErr      string
	}{
		{
			name:         "absent block yields defaults",
			content:      "[site_config]\nname = \"x\"\n",
			wantType:     "product",
			wantCurrency: "IDR",
			wantNotify:   "",
		},
		{
			name:         "explicit values win and currency uppercases",
			content:      "[catalog]\npost_type = \"merch\"\ncurrency = \"idr\"\n",
			wantType:     "merch",
			wantCurrency: "IDR",
		},
		{
			name:         "a notify address rides through trimmed",
			content:      "[catalog]\nnotify_email = \" ops@example.com \"\n",
			wantType:     "product",
			wantCurrency: "IDR",
			wantNotify:   "ops@example.com",
		},
		{
			name:         "blank post_type falls back to the default",
			content:      "[catalog]\npost_type = \"\"\ncurrency = \"IDR\"\n",
			wantType:     "product",
			wantCurrency: "IDR",
		},
		{
			name:    "lowercase-only currency normalized then validated",
			content: "[catalog]\ncurrency = \"us\"\n",
			wantErr: "3-letter uppercase code",
		},
		{
			name:    "non-alpha currency rejected",
			content: "[catalog]\ncurrency = \"12\"\n",
			wantErr: "3-letter uppercase code",
		},
		{
			name:         "success - display currency table parses",
			content:      "[catalog]\ncurrency = \"IDR\"\n[[catalog.display_currency]]\ncode = \"USD\"\nrate = 0.000062\n",
			wantType:     "product",
			wantCurrency: "IDR",
			wantDisplays: []config.DisplayCurrency{{Code: "USD", Rate: 0.000062}},
			wantErr:      "",
		},
		{
			name:         "success - display currencies inline list parses",
			content:      "[catalog]\ncurrency = \"IDR\"\ndisplay_currencies = [{code = \"USD\", rate = 0.000062}]\n",
			wantType:     "product",
			wantCurrency: "IDR",
			wantDisplays: []config.DisplayCurrency{{Code: "USD", Rate: 0.000062}},
			wantErr:      "",
		},
		{
			name:    "error - display rate zero names the field",
			content: "[catalog]\ncurrency = \"IDR\"\n[[catalog.display_currency]]\ncode = \"USD\"\nrate = 0\n",
			wantErr: "display_currency",
		},
		{
			name:    "error - non-finite display rate names the field",
			content: "[catalog]\ncurrency = \"IDR\"\n[[catalog.display_currency]]\ncode = \"USD\"\nrate = inf\n",
			wantErr: "display_currency",
		},
		{
			name:    "error - display code bad names the field",
			content: "[catalog]\ncurrency = \"IDR\"\n[[catalog.display_currency]]\ncode = \"US\"\nrate = 1\n",
			wantErr: "display_currency",
		},
		{
			name:    "error - display code equal to base names the field",
			content: "[catalog]\ncurrency = \"IDR\"\n[[catalog.display_currency]]\ncode = \"IDR\"\nrate = 1\n",
			wantErr: "display_currency",
		},
		{
			name:    "error - duplicate display code names the field",
			content: "[catalog]\ncurrency = \"IDR\"\n[[catalog.display_currency]]\ncode = \"USD\"\nrate = 1\n[[catalog.display_currency]]\ncode = \"usd\"\nrate = 2\n",
			wantErr: "display_currency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writeConfig(t, tt.content)

			cc, err := config.LoadCatalog(cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, cc.PostType)
			assert.Equal(t, tt.wantCurrency, cc.Currency)
			assert.Equal(t, tt.wantNotify, cc.NotifyEmail)
			assert.Equal(t, tt.wantDisplays, cc.DisplayCurrencies)
		})
	}
}

func TestLoadCatalog_NilConfig(t *testing.T) {
	_, err := config.LoadCatalog(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}
