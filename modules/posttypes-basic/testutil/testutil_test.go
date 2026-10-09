package testutil_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/posttypes-basic/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModule(t *testing.T) {
	module := testutil.NewModule()

	require.NotNil(t, module)
	assert.Equal(t, "posttypes-basic", module.Name())
	require.NotEmpty(t, module.PostTypes())
}

func TestAllowAll(t *testing.T) {
	assert.True(t, testutil.AllowAll(kernel.Principal{}, "admin"))
}

func TestKernel(t *testing.T) {
	host := testutil.Kernel()

	require.NotNil(t, host)
	assert.NotNil(t, host.Registry())
}
