package runtime_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// addWasm is a minimal WASM module: (func (export "add") (param i32 i32) (result i32) local.get 0 local.get 1 i32.add)
var addWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01,
	0x7f, 0x03, 0x02, 0x01, 0x00, 0x07, 0x07, 0x01,
	0x03, 0x61, 0x64, 0x64, 0x00, 0x00, 0x0a, 0x09,
	0x01, 0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a,
	0x0b,
}

// infiniteLoopWasm exports "infinite_loop" that loops forever.
var infiniteLoopWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00, 0x03, 0x02,
	0x01, 0x00, 0x07, 0x11, 0x01, 0x0d, 0x69, 0x6e,
	0x66, 0x69, 0x6e, 0x69, 0x74, 0x65, 0x5f, 0x6c,
	0x6f, 0x6f, 0x70, 0x00, 0x00, 0x0a, 0x09, 0x01,
	0x07, 0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b,
}

func newTestRuntime(t *testing.T, cfg runtime.RuntimeConfig) (runtime.Runtime, context.Context) {
	t.Helper()
	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })
	return rt, ctx
}

func compileAndInstantiate(t *testing.T, rt runtime.Runtime, ctx context.Context, wasm []byte, name string) api.Module {
	t.Helper()
	compiled, err := rt.Runtime().CompileModule(ctx, wasm)
	require.NoError(t, err)

	mod, err := rt.Runtime().InstantiateModule(
		ctx,
		compiled,
		wazero.NewModuleConfig().WithName(name),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mod.Close(ctx) })
	return mod
}

func TestExecuteFunc(t *testing.T) {
	t.Run("executes valid module function successfully", func(t *testing.T) {
		rt, ctx := newTestRuntime(t, runtime.RuntimeConfig{})
		mod := compileAndInstantiate(t, rt, ctx, addWasm, "test-add")

		result := rt.ExecuteFunc(ctx, mod, "add", 5, 3)

		require.NoError(t, result.Err)
		require.Len(t, result.Results, 1)
		assert.EqualValues(t, 8, result.Results[0])
	})

	t.Run("returns error for non-existent function", func(t *testing.T) {
		rt, ctx := newTestRuntime(t, runtime.RuntimeConfig{})
		mod := compileAndInstantiate(t, rt, ctx, addWasm, "test-add-func")

		result := rt.ExecuteFunc(ctx, mod, "nonexistent")

		require.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "not found")
	})

	t.Run("execution timeout logs error and returns error", func(t *testing.T) {
		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		rt, ctx := newTestRuntime(t, runtime.RuntimeConfig{
			MaxExecutionTime: 100 * time.Millisecond,
			Logger:           logger,
		})
		mod := compileAndInstantiate(t, rt, ctx, infiniteLoopWasm, "test-infinite")

		result := rt.ExecuteFunc(ctx, mod, "infinite_loop")

		require.Error(t, result.Err)
		assert.ErrorIs(t, result.Err, runtime.ErrResourceLimitsExceeded)
		assert.True(t, strings.Contains(logMessages[len(logMessages)-1], "Plugin exceeded resource limits"),
			"expected log message about resource limits, got: %v", logMessages)
	})

	t.Run("returns error when parent context is already canceled", func(t *testing.T) {
		rt, ctx := newTestRuntime(t, runtime.RuntimeConfig{})
		mod := compileAndInstantiate(t, rt, ctx, addWasm, "test-canceled-ctx")

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		result := rt.ExecuteFunc(cancelCtx, mod, "add", 1, 2)

		require.Error(t, result.Err)
		assert.NotErrorIs(t, result.Err, runtime.ErrResourceLimitsExceeded)
	})

	t.Run("runtime continues functioning after plugin exceeds limits", func(t *testing.T) {
		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		rt, ctx := newTestRuntime(t, runtime.RuntimeConfig{
			MaxExecutionTime: 100 * time.Millisecond,
			Logger:           logger,
		})

		// First: run infinite loop that should timeout
		infiniteMod := compileAndInstantiate(t, rt, ctx, infiniteLoopWasm, "test-infinite-1")
		result := rt.ExecuteFunc(ctx, infiniteMod, "infinite_loop")
		require.Error(t, result.Err)

		// Second: run a valid function — should still work
		addMod := compileAndInstantiate(t, rt, ctx, addWasm, "test-add-after")
		result = rt.ExecuteFunc(ctx, addMod, "add", 10, 20)
		require.NoError(t, result.Err)
		assert.EqualValues(t, 30, result.Results[0])
	})
}
