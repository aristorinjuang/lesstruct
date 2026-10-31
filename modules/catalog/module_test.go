package catalog_test

import (
	"io/fs"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/catalog"
	catalogapi "github.com/aristorinjuang/lesstruct/modules/catalog/api"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/aristorinjuang/lesstruct/modules/catalog/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, catalog.Name, module.Name())
	assert.Equal(t, catalog.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Empty(t, module.Optional())
	assert.NotEmpty(t, module.Capabilities())
	assert.Contains(t, module.Capabilities(), string(catalogdomain.CapabilityManage))
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
		assert.Contains(t, names, "000001_catalog.up.sql")
		assert.Contains(t, names, "000001_catalog.down.sql")
	}
}

func TestModule_Init_FailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		options []catalog.Option
		wantErr string
	}{
		{
			name:    "bare module fails on missing config",
			options: nil,
			wantErr: "[catalog] post_type is required",
		},
		{
			name: "unregistered product type fails closed",
			options: []catalog.Option{
				catalog.WithConfig(catalogapi.Config{PostType: "product", Currency: "IDR"}),
				catalog.WithPostTypes(denyPostTypes{}),
			},
			wantErr: `post type "product" is not registered`,
		},
		{
			name: "missing currency fails closed",
			options: []catalog.Option{
				catalog.WithConfig(catalogapi.Config{PostType: "product"}),
				catalog.WithPostTypes(stubPostTypes{}),
			},
			wantErr: "[catalog] currency is required",
		},
		{
			name: "registered product type passes",
			options: []catalog.Option{
				catalog.WithConfig(catalogapi.Config{PostType: "product", Currency: "IDR"}),
				catalog.WithPostTypes(stubPostTypes{}),
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := catalog.New(tt.options...)
			err := module.Init(testutil.Kernel())
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// stubPostTypes reports every slug as registered.
type stubPostTypes struct{}

func (stubPostTypes) Exists(string) bool { return true }

// denyPostTypes reports every slug as unregistered.
type denyPostTypes struct{}

func (denyPostTypes) Exists(string) bool { return false }
