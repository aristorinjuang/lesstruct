package seo_test

import (
	"io/fs"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/seo"
	"github.com/aristorinjuang/lesstruct/modules/seo/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, seo.Name, module.Name())
	assert.Equal(t, seo.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Empty(t, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_MigrationsLayout(t *testing.T) {
	module := testutil.NewModule()

	migrations := module.Migrations()
	require.NotNil(t, migrations)
	for _, dir := range []string{"sqlite", "postgresql", "mysql"} {
		entries, err := fs.ReadDir(migrations, dir)
		require.NoError(t, err, "driver dir %q must exist", dir)
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		assert.Contains(t, names, "000001_aliases.up.sql")
		assert.Contains(t, names, "000001_aliases.down.sql")
	}
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	assert.Empty(t, manifest.MenuEntries)
}

func TestModule_ImplementsMigrator(t *testing.T) {
	var _ kernel.Migrator = testutil.NewModule()
}

func TestModule_Handler(t *testing.T) {
	tests := []struct {
		name string
		opts []seo.Option
	}{
		{
			name: "success - bare module builds handler",
			opts: nil,
		},
		{
			name: "success - full options build configured handler",
			opts: []seo.Option{
				seo.WithPages(nil),
				seo.WithBaseURL("http://localhost:3000"),
				seo.WithHeadless(false),
				seo.WithLogger(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := seo.New(tt.opts...)

			require.NotNil(t, module.Handler())
		})
	}
}
