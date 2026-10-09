package modules_test

import (
	"io"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/aristorinjuang/lesstruct/kernel"
	usersmodule "github.com/aristorinjuang/lesstruct/modules/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleDependencies(t *testing.T) {
	tests := []struct {
		name     string
		module   kernel.Module
		expected string
		requires []string
		optional []string
	}{
		{
			name:     "users",
			module:   usersmodule.New(),
			expected: "users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.module.Name())
			assert.Equal(t, usersmodule.Version, tt.module.Version())
			assert.Equal(t, tt.requires, tt.module.Requires())
			assert.Equal(t, tt.optional, tt.module.Optional())
			assert.NoError(t, tt.module.Init(kernel.NewHost(nil)))
			assert.NoError(t, tt.module.Shutdown())
		})
	}
}

func TestDefaultEditionResolves(t *testing.T) {
	host := kernel.NewHost(nil)
	registry := host.Registry()
	for _, module := range modules.DefaultEdition() {
		require.NoError(t, registry.Register(module))
	}
	require.NoError(t, registry.InitAll(host))
	require.True(t, registry.Initialized())

	// All first-party wrappers are retired: every feature is a real module
	// registered explicitly by the composition root.
	assert.Empty(t, modules.DefaultEdition())

	require.NoError(t, registry.ShutdownAll())
}

func TestNewLogger(t *testing.T) {
	assert.Nil(t, modules.NewLogger(nil))

	logger := modules.NewLogger(util.NewLogger(io.Discard))
	require.NotNil(t, logger)
	assert.NotPanics(t, func() {
		logger.Info("hello %s", "modules")
		logger.Error("hello %s", "modules")
	})
}
