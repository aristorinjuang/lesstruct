package cart_test

import (
	"io/fs"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/cart"
	"github.com/aristorinjuang/lesstruct/modules/cart/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, cart.Name, module.Name())
	assert.Equal(t, cart.Version, module.Version())
	assert.Equal(t, []string{"catalog"}, module.Requires())
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
		assert.Contains(t, names, "000001_cart.up.sql")
		assert.Contains(t, names, "000001_cart.down.sql")
	}
}
