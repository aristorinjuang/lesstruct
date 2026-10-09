package importers_test

import (
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/modules/importers"
	"github.com/aristorinjuang/lesstruct/modules/importers/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, importers.Name, module.Name())
	assert.Equal(t, importers.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Equal(t, []string{"media", "posttypes-basic", "seo"}, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	require.Len(t, manifest.MenuEntries, 1)
	assert.Equal(t, "/import", manifest.MenuEntries[0].Path)
}

func TestModule_Handlers(t *testing.T) {
	tests := []struct {
		name string
		opts []importers.Option
	}{
		{
			name: "success - bare module builds handlers",
			opts: nil,
		},
		{
			name: "success - full options build configured handlers",
			opts: []importers.Option{
				importers.WithWordPressImporter(nil),
				importers.WithHugoImporter(nil),
				importers.WithMaxImportBytes(32 << 20),
				importers.WithWordPressTimeout(time.Minute),
				importers.WithHugoTimeout(time.Minute),
				importers.WithLogger(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := importers.New(tt.opts...)

			require.NotNil(t, module.WordPressHandler())
			require.NotNil(t, module.HugoHandler())
		})
	}
}
