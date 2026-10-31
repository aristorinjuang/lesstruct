package config_test

import (
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrders_NoConfigDir(t *testing.T) {
	cfg := &config.Config{
		ConfigDir:  "nonexistent-config-dir",
		ConfigFile: "config.toml",
	}

	oc, err := config.LoadOrders(cfg)
	require.NoError(t, err, "missing config dir should not error")
	assert.Equal(t, "24h", oc.PendingExpiry, "defaults must apply")

	window, err := oc.PendingExpiryDuration()
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, window)
}

func TestLoadOrders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr string
	}{
		{
			name:    "absent block yields defaults",
			content: "[site_config]\nname = \"x\"\n",
			want:    "24h",
		},
		{
			name:    "an explicit window wins",
			content: "[orders]\npending_expiry = \"2h\"\n",
			want:    "2h",
		},
		{
			name:    "a non-duration window refuses",
			content: "[orders]\npending_expiry = \"soon\"\n",
			wantErr: "pending_expiry \"soon\" is not a duration",
		},
		{
			name:    "malformed toml refuses",
			content: "[orders]\npending_expiry = 5\n",
			wantErr: "failed to parse orders config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writePaymentsConfig(t, tt.content)

			oc, err := config.LoadOrders(cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, oc.PendingExpiry)
		})
	}
}

func TestLoadOrders_NilConfig(t *testing.T) {
	_, err := config.LoadOrders(nil)
	require.Error(t, err)
}
