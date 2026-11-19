package admin_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/plugingen/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - manifest validates",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := admin.Manifest()
			require.NoError(t, manifest.Validate())
			assert.Len(t, manifest.MenuEntries, 2)
			assert.Equal(t, tt.wantErr, false)
		})
	}
}
