package registry_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
)

// hookWithMemoryWasm exports "hook_before_save" (identity on offset) + "memory" (2 pages)
// The hook function takes (offset i32, length i32) and returns the offset unchanged.
// Data flow: write data to memory at baseOffset -> call hook(baseOffset, len) -> returns baseOffset -> read from baseOffset
var hookWithMemoryWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f,
	0x03, 0x02, 0x01, 0x00,
	0x05, 0x03, 0x01, 0x00, 0x02,
	0x07, 0x1d, 0x02,
	0x10, 0x68, 0x6f, 0x6f, 0x6b, 0x5f, 0x62, 0x65, 0x66, 0x6f, 0x72, 0x65, 0x5f, 0x73, 0x61, 0x76, 0x65, 0x00, 0x00,
	0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00,
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x20, 0x00, 0x0b,
}

// hookReturnZeroWasm exports "hook_before_save" (returns 0) + "memory" (2 pages)
// Returns 0 which should be treated as nil result (AC5 no-return fallback)
var hookReturnZeroWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f,
	0x03, 0x02, 0x01, 0x00,
	0x05, 0x03, 0x01, 0x00, 0x02,
	0x07, 0x1d, 0x02,
	0x10, 0x68, 0x6f, 0x6f, 0x6b, 0x5f, 0x62, 0x65, 0x66, 0x6f, 0x72, 0x65, 0x5f, 0x73, 0x61, 0x76, 0x65, 0x00, 0x00,
	0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00,
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x41, 0x00, 0x0b,
}

func TestDiscoverHooksIntegration(t *testing.T) {
	t.Run("discovers and executes hook with memory bridge", func(t *testing.T) {
		ctx := t.Context()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		compiled, err := rt.Runtime().CompileModule(ctx, hookWithMemoryWasm)
		require.NoError(t, err)

		mod, err := rt.Runtime().InstantiateModule(
			ctx,
			compiled,
			wazero.NewModuleConfig().WithName("memory-plugin"),
		)
		require.NoError(t, err)

		plg := plugin.Plugin{
			Name:   "memory-plugin",
			Module: mod,
			Status: plugin.StatusLoaded,
		}

		reg := registry.NewRegistry(nil)

		err = registry.DiscoverHooks(ctx, plg, reg, rt, func(string) {})
		require.NoError(t, err)

		assert.True(t, reg.HasHook(plugin.HookBeforeSave))

		input := []byte("hello world")
		result, err := reg.Execute(ctx, plugin.HookBeforeSave, input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("data flows through WASM handler immutably", func(t *testing.T) {
		ctx := t.Context()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		compiled, err := rt.Runtime().CompileModule(ctx, hookWithMemoryWasm)
		require.NoError(t, err)

		mod, err := rt.Runtime().InstantiateModule(
			ctx,
			compiled,
			wazero.NewModuleConfig().WithName("immutable-plugin"),
		)
		require.NoError(t, err)

		plg := plugin.Plugin{
			Name:   "immutable-plugin",
			Module: mod,
			Status: plugin.StatusLoaded,
		}

		reg := registry.NewRegistry(nil)

		err = registry.DiscoverHooks(ctx, plg, reg, rt, func(string) {})
		require.NoError(t, err)

		original := []byte("immutable data")
		result, err := reg.Execute(ctx, plugin.HookBeforeSave, original)
		require.NoError(t, err)

		assert.Equal(t, []byte("immutable data"), original, "original Go data must be unchanged")
		assert.Equal(t, original, result)
	})

	t.Run("writing to WASM memory does not affect original Go data", func(t *testing.T) {
		ctx := t.Context()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		compiled, err := rt.Runtime().CompileModule(ctx, hookWithMemoryWasm)
		require.NoError(t, err)

		mod, err := rt.Runtime().InstantiateModule(
			ctx,
			compiled,
			wazero.NewModuleConfig().WithName("isolation-plugin"),
		)
		require.NoError(t, err)

		plg := plugin.Plugin{
			Name:   "isolation-plugin",
			Module: mod,
			Status: plugin.StatusLoaded,
		}

		reg := registry.NewRegistry(nil)

		err = registry.DiscoverHooks(ctx, plg, reg, rt, func(string) {})
		require.NoError(t, err)

		original := []byte("test isolation data with specific content")
		_, err = reg.Execute(ctx, plugin.HookBeforeSave, original)
		require.NoError(t, err)

		assert.Equal(
			t,
			[]byte("test isolation data with specific content"),
			original,
			"WASM memory writes must not affect Go-side data",
		)
	})

	t.Run("large data payloads", func(t *testing.T) {
		ctx := t.Context()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		compiled, err := rt.Runtime().CompileModule(ctx, hookWithMemoryWasm)
		require.NoError(t, err)

		mod, err := rt.Runtime().InstantiateModule(
			ctx,
			compiled,
			wazero.NewModuleConfig().WithName("large-payload-plugin"),
		)
		require.NoError(t, err)

		plg := plugin.Plugin{
			Name:   "large-payload-plugin",
			Module: mod,
			Status: plugin.StatusLoaded,
		}

		reg := registry.NewRegistry(nil)

		err = registry.DiscoverHooks(ctx, plg, reg, rt, func(string) {})
		require.NoError(t, err)

		largeData := make([]byte, 1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		result, err := reg.Execute(ctx, plugin.HookBeforeSave, largeData)
		require.NoError(t, err)
		assert.Equal(t, largeData, result)
	})

	t.Run("WASM function returning zero triggers no-return fallback", func(t *testing.T) {
		ctx := t.Context()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		compiled, err := rt.Runtime().CompileModule(ctx, hookReturnZeroWasm)
		require.NoError(t, err)

		mod, err := rt.Runtime().InstantiateModule(
			ctx,
			compiled,
			wazero.NewModuleConfig().WithName("zero-return-plugin"),
		)
		require.NoError(t, err)

		plg := plugin.Plugin{
			Name:   "zero-return-plugin",
			Module: mod,
			Status: plugin.StatusLoaded,
		}

		reg := registry.NewRegistry(nil)

		err = registry.DiscoverHooks(ctx, plg, reg, rt, func(string) {})
		require.NoError(t, err)

		input := []byte("fallback data")
		result, err := reg.Execute(ctx, plugin.HookBeforeSave, input)
		require.NoError(t, err)
		assert.Equal(t, []byte("fallback data"), result)
	})
}
