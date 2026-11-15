package textgen_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/textgen"
	"github.com/aristorinjuang/lesstruct/modules/textgen/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, textgen.Name, module.Name())
	assert.Equal(t, textgen.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Equal(t, []string{"media", "site-ssr"}, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	assert.Empty(t, manifest.MenuEntries)
}

func TestModule_RefreshThemeStyles(t *testing.T) {
	tests := []struct {
		name         string
		withRefresher bool
		baseCSS      string
		styleCSS     string
		wantBase     string
		wantStyle    string
	}{
		{
			name:          "success - forwards styles to refresher",
			withRefresher: true,
			baseCSS:       ":root{}",
			styleCSS:      ".btn{}",
			wantBase:      ":root{}",
			wantStyle:     ".btn{}",
		},
		{
			name:          "success - nil refresher is a no-op",
			withRefresher: false,
			baseCSS:       ":root{}",
			styleCSS:      ".btn{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBase, gotStyle string
			var opts []textgen.Option
			if tt.withRefresher {
				opts = append(opts, textgen.WithThemeRefresher(func(baseCSS, styleCSS string) {
					gotBase, gotStyle = baseCSS, styleCSS
				}))
			}
			module := textgen.New(opts...)
			require.NotNil(t, module)
			module.RefreshThemeStyles(tt.baseCSS, tt.styleCSS)
			if tt.withRefresher {
				assert.Equal(t, tt.wantBase, gotBase)
				assert.Equal(t, tt.wantStyle, gotStyle)
			}
		})
	}
}

func TestModule_Handlers(t *testing.T) {
	tests := []struct {
		name string
		opts []textgen.Option
	}{
		{
			name: "success - bare module builds handler",
			opts: nil,
		},
		{
			name: "success - full options build configured handler",
			opts: []textgen.Option{
				textgen.WithService(nil),
				textgen.WithMediaLister(nil),
				textgen.WithLogger(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := textgen.New(tt.opts...)

			require.NotNil(t, module.Handler())
			assert.Nil(t, module.Service())
		})
	}
}
