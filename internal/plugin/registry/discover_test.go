package registry_test

import (
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
)

// addWasm exports "add" as func (i32, i32) -> i32 (no hook_ prefix)
var addWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01,
	0x7f, 0x03, 0x02, 0x01, 0x00, 0x07, 0x07, 0x01,
	0x03, 0x61, 0x64, 0x64, 0x00, 0x00, 0x0a, 0x09,
	0x01, 0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a,
	0x0b,
}

// hookSaveWasm exports "hook_before_save" as func (i32, i32) -> i32 (identity: returns first param)
var hookSaveWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f,
	0x03, 0x02, 0x01, 0x00,
	0x07, 0x14, 0x01, 0x10,
	0x68, 0x6f, 0x6f, 0x6b, 0x5f, 0x62, 0x65, 0x66, 0x6f, 0x72, 0x65, 0x5f, 0x73, 0x61, 0x76, 0x65,
	0x00, 0x00,
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x20, 0x00, 0x0b,
}

// hookMultiWasm exports "hook_before_save" and "hook_after_publish" as identity functions
var hookMultiWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f,
	0x03, 0x03, 0x02, 0x00, 0x00,
	0x07, 0x29, 0x02,
	0x10, 0x68, 0x6f, 0x6f, 0x6b, 0x5f, 0x62, 0x65, 0x66, 0x6f, 0x72, 0x65, 0x5f, 0x73, 0x61, 0x76, 0x65, 0x00, 0x00,
	0x12, 0x68, 0x6f, 0x6f, 0x6b, 0x5f, 0x61, 0x66, 0x74, 0x65, 0x72, 0x5f, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x73, 0x68, 0x00, 0x01,
	0x0a, 0x0b, 0x02,
	0x04, 0x00, 0x20, 0x00, 0x0b,
	0x04, 0x00, 0x20, 0x00, 0x0b,
}

func compileAndInstantiate(
	t *testing.T,
	rt runtime.Runtime,
	wasm []byte,
	name string,
) plugin.Plugin {
	t.Helper()
	ctx := t.Context()

	compiled, err := rt.Runtime().CompileModule(ctx, wasm)
	require.NoError(t, err)

	mod, err := rt.Runtime().InstantiateModule(
		ctx,
		compiled,
		wazero.NewModuleConfig().WithName(name),
	)
	require.NoError(t, err)

	return plugin.Plugin{
		Name:     name,
		Module:   mod,
		Status:   plugin.StatusLoaded,
		LoadedAt: time.Now(),
	}
}

func newTestRuntime(t *testing.T) runtime.Runtime {
	t.Helper()
	ctx := t.Context()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })
	return rt
}

func TestDiscoverHooksDiscoversHookFunctions(t *testing.T) {
	rt := newTestRuntime(t)
	reg := registry.NewRegistry(nil)
	var logOutput string

	plg := compileAndInstantiate(t, rt, hookSaveWasm, "test-plugin")

	err := registry.DiscoverHooks(
		t.Context(),
		plg,
		reg,
		rt,
		func(msg string) { logOutput += msg + "\n" },
	)
	require.NoError(t, err)

	assert.True(t, reg.HasHook(plugin.HookBeforeSave))
	assert.Contains(t, logOutput, "Registered hooks: before_save")
}

func TestDiscoverHooksIgnoresNonHookExports(t *testing.T) {
	rt := newTestRuntime(t)
	reg := registry.NewRegistry(nil)
	var logOutput string

	plg := compileAndInstantiate(t, rt, addWasm, "test-plugin")

	err := registry.DiscoverHooks(
		t.Context(),
		plg,
		reg,
		rt,
		func(msg string) { logOutput += msg + "\n" },
	)
	require.NoError(t, err)

	assert.False(t, reg.HasHook(plugin.HookName("add")))
	assert.False(t, strings.Contains(logOutput, "Registered hooks"))
}

func TestDiscoverHooksNoHookExports(t *testing.T) {
	rt := newTestRuntime(t)
	reg := registry.NewRegistry(nil)
	var logOutput string

	plg := compileAndInstantiate(t, rt, addWasm, "no-hooks-plugin")

	err := registry.DiscoverHooks(
		t.Context(),
		plg,
		reg,
		rt,
		func(msg string) { logOutput += msg + "\n" },
	)
	require.NoError(t, err)

	hooks := reg.Hooks()
	assert.Empty(t, hooks)
	assert.Empty(t, logOutput)
}

func TestDiscoverHooksDefaultPriority(t *testing.T) {
	rt := newTestRuntime(t)
	reg := registry.NewRegistry(nil)

	plg := compileAndInstantiate(t, rt, hookSaveWasm, "test-plugin")

	err := registry.DiscoverHooks(
		t.Context(),
		plg,
		reg,
		rt,
		func(string) {},
	)
	require.NoError(t, err)

	hooks := reg.Hooks()
	require.Len(t, hooks[plugin.HookBeforeSave], 1)
	assert.Equal(t, plugin.DefaultPriority, hooks[plugin.HookBeforeSave][0].Priority)
}

func TestDiscoverHooksDefaultFailureMode(t *testing.T) {
	rt := newTestRuntime(t)
	reg := registry.NewRegistry(nil)

	plg := compileAndInstantiate(t, rt, hookSaveWasm, "test-plugin")

	err := registry.DiscoverHooks(
		t.Context(),
		plg,
		reg,
		rt,
		func(string) {},
	)
	require.NoError(t, err)

	hooks := reg.Hooks()
	require.Len(t, hooks[plugin.HookBeforeSave], 1)
	assert.Equal(t, plugin.DefaultFailureMode, hooks[plugin.HookBeforeSave][0].FailureMode)
}

func TestDiscoverHooksMultipleHooks(t *testing.T) {
	rt := newTestRuntime(t)
	reg := registry.NewRegistry(nil)
	var logOutput string

	plg := compileAndInstantiate(t, rt, hookMultiWasm, "test-plugin")

	err := registry.DiscoverHooks(
		t.Context(),
		plg,
		reg,
		rt,
		func(msg string) { logOutput += msg + "\n" },
	)
	require.NoError(t, err)

	assert.True(t, reg.HasHook(plugin.HookBeforeSave))
	assert.True(t, reg.HasHook(plugin.HookAfterPublish))
	assert.Contains(t, logOutput, "Registered hooks: after_publish, before_save")
}
