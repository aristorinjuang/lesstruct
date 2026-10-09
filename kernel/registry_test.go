package kernel_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubModule struct {
	name        string
	requires    []string
	optional    []string
	initErr     error
	shutdownErr error
	calls       *[]string
}

func (m *stubModule) Name() string {
	return m.name
}

func (m *stubModule) Version() string {
	return "test"
}

func (m *stubModule) Requires() []string {
	return m.requires
}

func (m *stubModule) Optional() []string {
	return m.optional
}

func (m *stubModule) Init(_ kernel.Kernel) error {
	if m.calls != nil {
		*m.calls = append(*m.calls, "init:"+m.name)
	}
	return m.initErr
}

func (m *stubModule) Shutdown() error {
	if m.calls != nil {
		*m.calls = append(*m.calls, "shutdown:"+m.name)
	}
	return m.shutdownErr
}

func newStubModule(
	name string,
	requires []string,
	calls *[]string,
) *stubModule {
	return &stubModule{
		name:     name,
		requires: requires,
		calls:    calls,
	}
}

type stubCapper interface {
	kernel.Module
	Cap() string
}

type stubCappableModule struct {
	stubModule
	capability string
}

func (m *stubCappableModule) Cap() string {
	return m.capability
}

func TestRegistryRegister(t *testing.T) {
	tests := []struct {
		name    string
		modules []kernel.Module
		wantErr bool
		errIs   error
	}{
		{
			name:    "success - distinct modules",
			modules: []kernel.Module{newStubModule("a", nil, nil), newStubModule("b", nil, nil)},
			wantErr: false,
		},
		{
			name:    "error - duplicate name",
			modules: []kernel.Module{newStubModule("a", nil, nil), newStubModule("a", nil, nil)},
			wantErr: true,
			errIs:   kernel.ErrDuplicateModule,
		},
		{
			name:    "error - nil module",
			modules: []kernel.Module{nil},
			wantErr: true,
			errIs:   kernel.ErrNilModule,
		},
		{
			name:    "error - empty name",
			modules: []kernel.Module{newStubModule("", nil, nil)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := kernel.NewRegistry()
			var err error
			for _, module := range tt.modules {
				err = registry.Register(module)
				if err != nil {
					break
				}
			}
			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRegistryInitOrder(t *testing.T) {
	tests := []struct {
		name          string
		registerOrder []string
		requires      map[string][]string
		expected      []string
	}{
		{
			name:          "success - dependency order regardless of registration order",
			registerOrder: []string{"c", "a", "b"},
			requires: map[string][]string{
				"b": {"a"},
				"c": {"b"},
			},
			expected: []string{"init:a", "init:b", "init:c"},
		},
		{
			name:          "success - independent modules sort by name",
			registerOrder: []string{"b", "a"},
			requires:      map[string][]string{},
			expected:      []string{"init:a", "init:b"},
		},
		{
			name:          "success - diamond resolves once",
			registerOrder: []string{"d", "c", "b", "a"},
			requires: map[string][]string{
				"b": {"a"},
				"c": {"a"},
				"d": {"b", "c"},
			},
			expected: []string{"init:a", "init:b", "init:c", "init:d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			registry := kernel.NewRegistry()
			host := kernel.NewHost(nil)
			for _, name := range tt.registerOrder {
				require.NoError(t, registry.Register(newStubModule(name, tt.requires[name], &calls)))
			}
			require.NoError(t, registry.InitAll(host))
			assert.Equal(t, tt.expected, calls)
			assert.True(t, registry.Initialized())
			expectedOrder := make([]string, 0, len(tt.expected))
			for _, call := range tt.expected {
				expectedOrder = append(expectedOrder, strings.TrimPrefix(call, "init:"))
			}
			assert.Equal(t, expectedOrder, registry.InitOrder())
		})
	}
}

func TestRegistryInitMissingDependency(t *testing.T) {
	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(newStubModule("comments", []string{"users"}, nil)))

	err := registry.InitAll(kernel.NewHost(nil))
	require.Error(t, err)
	assert.ErrorIs(t, err, kernel.ErrMissingDependency)
	assert.False(t, registry.Initialized())
}

func TestRegistryInitOptionalMissing(t *testing.T) {
	var calls []string
	registry := kernel.NewRegistry()
	module := newStubModule("textgen", []string{"media"}, &calls)
	module.optional = []string{"media"}
	require.NoError(t, registry.Register(module))

	require.NoError(t, registry.InitAll(kernel.NewHost(nil)))
	assert.Equal(t, []string{"init:textgen"}, calls)
	assert.True(t, registry.Initialized())
}

func TestRegistryInitCircular(t *testing.T) {
	tests := []struct {
		name     string
		requires map[string][]string
	}{
		{
			name: "error - direct cycle",
			requires: map[string][]string{
				"a": {"b"},
				"b": {"a"},
			},
		},
		{
			name: "error - self dependency",
			requires: map[string][]string{
				"a": {"a"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := kernel.NewRegistry()
			for name, requires := range tt.requires {
				require.NoError(t, registry.Register(newStubModule(name, requires, nil)))
			}
			err := registry.InitAll(kernel.NewHost(nil))
			require.Error(t, err)
			assert.ErrorIs(t, err, kernel.ErrCircularDependency)
			assert.False(t, registry.Initialized())
		})
	}
}

func TestRegistryInitErrorAborts(t *testing.T) {
	var calls []string
	registry := kernel.NewRegistry()
	failing := newStubModule("a", nil, &calls)
	failing.initErr = errors.New("init failed")
	require.NoError(t, registry.Register(failing))
	require.NoError(t, registry.Register(newStubModule("b", []string{"a"}, &calls)))

	err := registry.InitAll(kernel.NewHost(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"a"`)
	assert.Equal(t, []string{"init:a"}, calls)
	assert.False(t, registry.Initialized())
}

func TestRegistryInitNilKernel(t *testing.T) {
	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(newStubModule("a", nil, nil)))

	err := registry.InitAll(nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, kernel.ErrNilKernel)
}

func TestRegistryShutdownReverse(t *testing.T) {
	var calls []string
	registry := kernel.NewRegistry()
	host := kernel.NewHost(nil)
	require.NoError(t, registry.Register(newStubModule("a", nil, &calls)))
	require.NoError(t, registry.Register(newStubModule("b", []string{"a"}, &calls)))
	require.NoError(t, registry.Register(newStubModule("c", []string{"b"}, &calls)))
	require.NoError(t, registry.InitAll(host))

	calls = nil
	require.NoError(t, registry.ShutdownAll())
	assert.Equal(t, []string{"shutdown:c", "shutdown:b", "shutdown:a"}, calls)
}

func TestRegistryShutdownCollectsErrors(t *testing.T) {
	registry := kernel.NewRegistry()
	host := kernel.NewHost(nil)
	failing := newStubModule("a", nil, nil)
	failing.shutdownErr = errors.New("shutdown failed")
	require.NoError(t, registry.Register(failing))
	require.NoError(t, registry.Register(newStubModule("b", []string{"a"}, nil)))
	require.NoError(t, registry.InitAll(host))

	err := registry.ShutdownAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"a"`)
}

func TestRegistryShutdownWithoutInit(t *testing.T) {
	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(newStubModule("a", nil, nil)))

	assert.NoError(t, registry.ShutdownAll())
}

func TestRegistryGetAndNames(t *testing.T) {
	registry := kernel.NewRegistry()
	require.NoError(t, registry.Register(newStubModule("b", nil, nil)))
	require.NoError(t, registry.Register(newStubModule("a", nil, nil)))

	assert.Equal(t, []string{"a", "b"}, registry.Names())

	module, ok := registry.Get("a")
	require.True(t, ok)
	assert.Equal(t, "a", module.Name())

	_, ok = registry.Get("missing")
	assert.False(t, ok)
}

func TestCapabilitiesDiscovery(t *testing.T) {
	registry := kernel.NewRegistry()
	plain := newStubModule("plain", nil, nil)
	capped := &stubCappableModule{stubModule: stubModule{name: "capped"}, capability: "yes"}
	require.NoError(t, registry.Register(plain))
	require.NoError(t, registry.Register(capped))

	found := kernel.Capabilities[stubCapper](registry)
	require.Len(t, found, 1)
	assert.Equal(t, "yes", found[0].Cap())
	assert.Equal(t, "capped", found[0].Name())
}

func TestHostDefaults(t *testing.T) {
	host := kernel.NewHost(nil)
	assert.NotNil(t, host.Logger())
	assert.NotNil(t, host.Registry())
}
