package admin_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/media/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	manifest := admin.Manifest()

	require.Len(t, manifest.MenuEntries, 1)
	assert.Equal(t, "Media", manifest.MenuEntries[0].Label)
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
			name:     "success - module manifest validates",
			manifest: admin.Manifest(),
			wantErr:  false,
		},
		{
			name: "error - entry without label",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Path: "/media"}},
			},
			wantErr: true,
		},
		{
			name: "error - entry without path",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Label: "Media"}},
			},
			wantErr: true,
		},
		{
			name: "error - relative path",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Label: "Media", Path: "media"}},
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
