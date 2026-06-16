package loader_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/loader"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLoader(t *testing.T, pluginsDir string) (loader.Loader, context.Context) {
	t.Helper()
	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{
		Logger: func(string) {},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	l := loader.NewLoader(rt, pluginsDir, nil, func(string) {})
	return l, ctx
}

// addWasm is a minimal WASM module: (func (export "add") (param i32 i32) (result i32))
var addWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01,
	0x7f, 0x03, 0x02, 0x01, 0x00, 0x07, 0x07, 0x01,
	0x03, 0x61, 0x64, 0x64, 0x00, 0x00, 0x0a, 0x09,
	0x01, 0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a,
	0x0b,
}

func TestEnsurePluginsDir(t *testing.T) {
	t.Run("creates missing directory", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")

		l, _ := newTestLoader(t, pluginsDir)

		err := l.EnsurePluginsDir()
		require.NoError(t, err)

		info, err := os.Stat(pluginsDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("idempotent when directory exists", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		l, _ := newTestLoader(t, pluginsDir)

		err := l.EnsurePluginsDir()
		require.NoError(t, err)
	})

	t.Run("logs creation message", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")

		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		l := loader.NewLoader(rt, pluginsDir, nil, logger)

		err = l.EnsurePluginsDir()
		require.NoError(t, err)
		assert.Contains(t, logMessages, "Created plugins/ directory")
	})
}

func TestDiscover(t *testing.T) {
	t.Run("finds wasm files", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "plugin1.wasm"),
			addWasm,
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "plugin2.wasm"),
			addWasm,
			0o644,
		))

		l, ctx := newTestLoader(t, pluginsDir)

		paths, err := l.Discover(ctx)
		require.NoError(t, err)
		assert.Len(t, paths, 2)
	})

	t.Run("ignores non-wasm files", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "valid.wasm"),
			addWasm,
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "readme.txt"),
			[]byte("not wasm"),
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "data.json"),
			[]byte("{}"),
			0o644,
		))

		l, ctx := newTestLoader(t, pluginsDir)

		paths, err := l.Discover(ctx)
		require.NoError(t, err)
		assert.Len(t, paths, 1)
		assert.Contains(t, paths[0], "valid.wasm")
	})

	t.Run("ignores directories", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(pluginsDir, "subdir"), 0o755))

		l, ctx := newTestLoader(t, pluginsDir)

		paths, err := l.Discover(ctx)
		require.NoError(t, err)
		assert.Empty(t, paths)
	})

	t.Run("returns empty list for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		l, ctx := newTestLoader(t, pluginsDir)

		paths, err := l.Discover(ctx)
		require.NoError(t, err)
		assert.Empty(t, paths)
	})
}

func TestLoadAll(t *testing.T) {
	t.Run("loads all valid plugins", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "add.wasm"),
			addWasm,
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "math.wasm"),
			addWasm,
			0o644,
		))

		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{
			Logger: logger,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		l := loader.NewLoader(rt, pluginsDir, nil, logger)

		loaded, results := l.LoadAll(ctx)
		require.Len(t, loaded, 2)
		assert.Len(t, results, 2)
		assert.Contains(t, logMessages, "Loaded plugin: add.wasm")
		assert.Contains(t, logMessages, "Loaded plugin: math.wasm")
	})

	t.Run("skips corrupted plugins and continues", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "valid.wasm"),
			addWasm,
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "corrupted.wasm"),
			[]byte{0x00, 0x01, 0x02, 0x03},
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "also-valid.wasm"),
			addWasm,
			0o644,
		))

		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{
			Logger: logger,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		l := loader.NewLoader(rt, pluginsDir, nil, logger)

		loaded, results := l.LoadAll(ctx)
		require.Len(t, loaded, 2)
		assert.Len(t, results, 3)

		assert.Contains(t, logMessages, "Loaded plugin: valid.wasm")
		assert.Contains(t, logMessages, "Loaded plugin: also-valid.wasm")

		var foundCorrupted bool
		for _, msg := range logMessages {
			if strings.Contains(msg, "Failed to load plugin: corrupted.wasm") {
				foundCorrupted = true
			}
		}
		assert.True(t, foundCorrupted, "expected error log for corrupted plugin")
	})

	t.Run("one plugin failure does not affect others", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "bad.wasm"),
			[]byte{0xDE, 0xAD, 0xBE, 0xEF},
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "good.wasm"),
			addWasm,
			0o644,
		))

		l, ctx := newTestLoader(t, pluginsDir)

		loaded, results := l.LoadAll(ctx)
		require.Len(t, loaded, 1)
		assert.Equal(t, "good", loaded[0].Name)
		assert.Len(t, results, 2)

		var hasError, hasSuccess bool
		for _, r := range results {
			if r.Err != nil {
				hasError = true
			} else {
				hasSuccess = true
			}
		}
		assert.True(t, hasError, "expected one plugin to fail")
		assert.True(t, hasSuccess, "expected one plugin to succeed")
	})

	t.Run("loads from empty directory", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		l, ctx := newTestLoader(t, pluginsDir)

		loaded, results := l.LoadAll(ctx)
		assert.Empty(t, loaded)
		assert.Empty(t, results)
	})

	t.Run("loaded plugins have correct metadata", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "my-plugin.wasm"),
			addWasm,
			0o644,
		))

		before := time.Now()
		l, ctx := newTestLoader(t, pluginsDir)

		loaded, _ := l.LoadAll(ctx)
		require.Len(t, loaded, 1)

		p := loaded[0]
		assert.Equal(t, "my-plugin", p.Name)
		assert.Contains(t, p.FilePath, "my-plugin.wasm")
		assert.Equal(t, plugin.StatusLoaded, p.Status)
		assert.False(t, p.LoadedAt.Before(before))
		assert.NotNil(t, p.Module)
	})
}

func TestLoadAllCrossPlatformLog(t *testing.T) {
	t.Run("logs cross-platform message for multiple plugins", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "a.wasm"),
			addWasm,
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "b.wasm"),
			addWasm,
			0o644,
		))

		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{Logger: logger})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		l := loader.NewLoader(rt, pluginsDir, nil, logger)
		l.LoadAll(ctx)

		var found bool
		for _, msg := range logMessages {
			if strings.Contains(msg, "cross-platform") {
				found = true
			}
		}
		assert.True(t, found, "expected cross-platform log message")
	})

	t.Run("logs cross-platform message for single plugin", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "single.wasm"),
			addWasm,
			0o644,
		))

		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{Logger: logger})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		l := loader.NewLoader(rt, pluginsDir, nil, logger)
		l.LoadAll(ctx)

		var found bool
		for _, msg := range logMessages {
			if strings.Contains(msg, "cross-platform") {
				found = true
			}
		}
		assert.True(t, found, "expected cross-platform log for single plugin")
	})
}

func TestLoad(t *testing.T) {
	t.Run("loads a single valid wasm plugin", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		wasmPath := filepath.Join(pluginsDir, "math.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		l, ctx := newTestLoader(t, pluginsDir)

		plg, manifest, err := l.Load(ctx, wasmPath)
		require.NoError(t, err)
		assert.Nil(t, manifest)
		assert.Equal(t, "math", plg.Name)
		assert.Equal(t, plugin.StatusLoaded, plg.Status)
		assert.NotNil(t, plg.Module)
	})

	t.Run("returns error for corrupted wasm file", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		wasmPath := filepath.Join(pluginsDir, "bad.wasm")
		require.NoError(t, os.WriteFile(wasmPath, []byte{0xDE, 0xAD}, 0o644))

		l, ctx := newTestLoader(t, pluginsDir)

		plg, manifest, err := l.Load(ctx, wasmPath)
		require.Error(t, err)
		assert.Nil(t, manifest)
		assert.Equal(t, plugin.StatusFailed, plg.Status)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		l, ctx := newTestLoader(t, pluginsDir)

		plg, manifest, err := l.Load(ctx, "/nonexistent/plugin.wasm")
		require.Error(t, err)
		assert.Nil(t, manifest)
		assert.Equal(t, plugin.StatusFailed, plg.Status)
	})

	t.Run("loaded plugin has correct metadata", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		wasmPath := filepath.Join(pluginsDir, "my-plugin.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		before := time.Now()
		l, ctx := newTestLoader(t, pluginsDir)

		plg, manifest, err := l.Load(ctx, wasmPath)
		require.NoError(t, err)
		assert.Nil(t, manifest)

		assert.Equal(t, "my-plugin", plg.Name)
		assert.Contains(t, plg.FilePath, "my-plugin.wasm")
		assert.Equal(t, plugin.StatusLoaded, plg.Status)
		assert.False(t, plg.LoadedAt.Before(before))
		assert.NotNil(t, plg.Module)
	})
}

func TestUnload(t *testing.T) {
	t.Run("closes a loaded plugin module", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		wasmPath := filepath.Join(pluginsDir, "math.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		l, ctx := newTestLoader(t, pluginsDir)

		plg, _, err := l.Load(ctx, wasmPath)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusLoaded, plg.Status)
		assert.NotNil(t, plg.Module)

		err = l.Unload(ctx, &plg)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusUnloaded, plg.Status)
	})

	t.Run("returns error for nil module plugin", func(t *testing.T) {
		l, _ := newTestLoader(t, t.TempDir())

		plg := plugin.Plugin{Name: "empty", Module: nil}
		err := l.Unload(context.Background(), &plg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "module is nil")
	})
}

func TestNewLoader(t *testing.T) {
	t.Run("uses nil logger as no-op", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "test.wasm"),
			addWasm,
			0o644,
		))

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		l := loader.NewLoader(rt, pluginsDir, nil, nil)

		loaded, _ := l.LoadAll(ctx)
		assert.Len(t, loaded, 1)
	})

	t.Run("uses nil host functions without panic", func(t *testing.T) {
		dir := t.TempDir()
		pluginsDir := filepath.Join(dir, "plugins")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(pluginsDir, "test.wasm"),
			addWasm,
			0o644,
		))

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		l := loader.NewLoader(rt, pluginsDir, nil, func(string) {})

		loaded, _ := l.LoadAll(ctx)
		assert.Len(t, loaded, 1)
	})
}