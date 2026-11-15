// Package admin_test covers the themegen admin manifest.
package admin_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/themegen/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "success - manifest validates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := admin.Manifest()
			require.NoError(t, manifest.Validate())
			assert.Len(t, manifest.MenuEntries, 2)
		})
	}
}
