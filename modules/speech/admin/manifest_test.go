package admin_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/speech/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	manifest := admin.Manifest()

	assert.Empty(t, manifest.MenuEntries)
	assert.Empty(t, manifest.SettingsSections)
	assert.Empty(t, manifest.EditorExtensions)
	require.NoError(t, manifest.Validate())
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		manifest kernel.AdminExtensionManifest
		wantErr  bool
	}{
		{
			name:     "success - empty manifest validates",
			manifest: kernel.AdminExtensionManifest{},
			wantErr:  false,
		},
		{
			name: "success - well-formed entry validates",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Label: "Speech", Path: "/speech"}},
			},
			wantErr: false,
		},
		{
			name: "error - entry without label",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Path: "/speech"}},
			},
			wantErr: true,
		},
		{
			name: "error - entry without path",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Label: "Speech"}},
			},
			wantErr: true,
		},
		{
			name: "error - relative path",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Label: "Speech", Path: "speech"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
