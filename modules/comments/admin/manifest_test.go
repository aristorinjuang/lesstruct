package admin_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/comments/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	manifest := admin.Manifest()

	require.Len(t, manifest.MenuEntries, 1)
	assert.Equal(t, "Comments", manifest.MenuEntries[0].Label)
	assert.Equal(t, "/comment", manifest.MenuEntries[0].Path)
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
				MenuEntries: []kernel.AdminMenuEntry{{Path: "/comment"}},
			},
			wantErr: true,
		},
		{
			name: "error - entry without path",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Label: "Comments"}},
			},
			wantErr: true,
		},
		{
			name: "error - relative path",
			manifest: kernel.AdminExtensionManifest{
				MenuEntries: []kernel.AdminMenuEntry{{Label: "Comments", Path: "comment"}},
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
