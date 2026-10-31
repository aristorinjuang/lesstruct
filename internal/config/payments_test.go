package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePaymentsConfig(t *testing.T, content string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o600))
	return &config.Config{ConfigDir: dir, ConfigFile: "config.toml"}
}

func TestLoadPayments_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	pc, err := config.LoadPayments(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Equal(t, []string{"manual"}, pc.Methods, "defaults must apply")
}

func TestLoadPayments_NoFile(t *testing.T) {
	cfg := &config.Config{ConfigDir: t.TempDir(), ConfigFile: "config.toml"}

	pc, err := config.LoadPayments(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"manual"}, pc.Methods)
}

func TestLoadPayments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
		wantErr string
	}{
		{
			name:    "absent block yields defaults",
			content: "[site_config]\nname = \"x\"\n",
			want:    []string{"manual"},
		},
		{
			name:    "explicit values win",
			content: "[payments]\nmethods = [\"midtrans\", \"manual\"]\n",
			want:    []string{"midtrans", "manual"},
		},
		{
			name:    "values trim and lowercase",
			content: "[payments]\nmethods = [\" Manual \"]\n",
			want:    []string{"manual"},
		},
		{
			name:    "an all-blank list yields defaults",
			content: "[payments]\nmethods = [\"\", \"   \"]\n",
			want:    []string{"manual"},
		},
		{
			name:    "unknown methods ride through for the composition root to drop",
			content: "[payments]\nmethods = [\"futuregateway\"]\n",
			want:    []string{"futuregateway"},
		},
		{
			name:    "malformed toml refuses",
			content: "[payments]\nmethods = \"not-a-list\"\n",
			wantErr: "failed to parse payments config",
		},
		{
			name:    "a path-skipping config file refuses",
			content: "unused",
			want:    nil,
			wantErr: "CONFIG_FILE must not contain path separators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writePaymentsConfig(t, tt.content)
			if tt.wantErr == "CONFIG_FILE must not contain path separators" {
				cfg.ConfigFile = "../escape.toml"
			}

			pc, err := config.LoadPayments(cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, pc.Methods)
		})
	}
}

func TestLoadPayments_Manual(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantManual config.ManualPayee
		wantErr    string
	}{
		{
			name: "absent block yields zero payee",
			content: `[payments]
methods = ["manual"]
`,
			wantManual: config.ManualPayee{},
		},
		{
			name: "full manual block parses and trims",
			content: `[payments]
methods = ["manual"]

[payments.manual]
bank = " BCA "
account = "1234567890"
name = " PT Demo "
`,
			wantManual: config.ManualPayee{Bank: "BCA", Account: "1234567890", Name: "PT Demo"},
		},
		{
			name: "missing bank fails closed naming the field",
			content: `[payments.manual]
account = "1234567890"
name = "PT Demo"
`,
			wantErr: "bank",
		},
		{
			name: "missing account fails closed naming the field",
			content: `[payments.manual]
bank = "BCA"
name = "PT Demo"
`,
			wantErr: "account",
		},
		{
			name: "missing name fails closed naming the field",
			content: `[payments.manual]
bank = "BCA"
account = "1234567890"
`,
			wantErr: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writePaymentsConfig(t, tt.content)

			pc, err := config.LoadPayments(cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "incomplete [payments.manual] block")
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantManual, pc.Manual)
		})
	}
}

func TestLoadPayments_NilConfig(t *testing.T) {
	_, err := config.LoadPayments(nil)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "config cannot be nil"))
}

func TestLoadPayments_MidtransEnv(t *testing.T) {
	t.Run("the server key and production flag ride the environment", func(t *testing.T) {
		t.Setenv("MIDTRANS_SERVER_KEY", "SB-Mid-server-secret")
		t.Setenv("MIDTRANS_IS_PRODUCTION", "true")
		cfg := &config.Config{ConfigDir: t.TempDir(), ConfigFile: "config.toml"}

		pc, err := config.LoadPayments(cfg)

		require.NoError(t, err)
		assert.Equal(t, "SB-Mid-server-secret", pc.MidtransServerKey)
		assert.True(t, pc.MidtransIsProduction)
	})

	t.Run("an unset key leaves the gateway unadopted", func(t *testing.T) {
		cfg := &config.Config{ConfigDir: t.TempDir(), ConfigFile: "config.toml"}

		pc, err := config.LoadPayments(cfg)

		require.NoError(t, err)
		assert.Equal(t, "", pc.MidtransServerKey)
		assert.False(t, pc.MidtransIsProduction)
	})
}
