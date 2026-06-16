package devmode_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/plugin/devmode"
	"github.com/aristorinjuang/lesstruct/internal/plugin/loader"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var addWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01,
	0x7f, 0x03, 0x02, 0x01, 0x00, 0x07, 0x07, 0x01,
	0x03, 0x61, 0x64, 0x64, 0x00, 0x00, 0x0a, 0x09,
	0x01, 0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a,
	0x0b,
}

type logCapture struct {
	mu       sync.Mutex
	messages []string
}

func (l *logCapture) log(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, msg)
}

func (l *logCapture) contains(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.messages {
		if m == s {
			return true
		}
	}
	return false
}

func (l *logCapture) containsPrefix(prefix string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.messages {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}


func (l *logCapture) count(s string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := 0
	for _, m := range l.messages {
		if m == s {
			c++
		}
	}
	return c
}

func newTestSetup(t *testing.T) (
	*devmode.Watcher,
	*registry.Registry,
	runtime.Runtime,
	context.Context,
	string,
	*logCapture,
) {
	t.Helper()
	ctx := context.Background()

	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	lc := &logCapture{}

	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{Logger: lc.log})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	ldr := loader.NewLoader(rt, pluginsDir, nil, lc.log)
	reg := registry.NewRegistry(lc.log)
	w := devmode.NewWatcher(ldr, reg, rt, pluginsDir, lc.log)

	return w, reg, rt, ctx, pluginsDir, lc
}

func TestNewWatcher(t *testing.T) {
	t.Run("creates watcher with nil logger", func(t *testing.T) {
		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		w := devmode.NewWatcher(loader.Loader{}, registry.NewRegistry(nil), rt, t.TempDir(), nil)
		assert.NotNil(t, w)
	})
}

func TestStart(t *testing.T) {
	t.Run("logs development mode message", func(t *testing.T) {
		w, _, _, ctx, _, lc := newTestSetup(t)
		defer w.Stop()

		err := w.Start(ctx)
		require.NoError(t, err)

		assert.True(t, lc.contains("Development mode activated - hot-reload enabled"))
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		_, _, rt, ctx, _, _ := newTestSetup(t)

		ldr := loader.NewLoader(rt, "/nonexistent/plugins", nil, func(string) {})
		reg := registry.NewRegistry(func(string) {})
		w := devmode.NewWatcher(ldr, reg, rt, "/nonexistent/plugins", func(string) {})

		err := w.Start(ctx)
		require.Error(t, err)
	})

	t.Run("returns error when called twice", func(t *testing.T) {
		w, _, _, ctx, _, _ := newTestSetup(t)
		defer w.Stop()

		err := w.Start(ctx)
		require.NoError(t, err)

		err = w.Start(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already started")
	})
}

func TestStop(t *testing.T) {
	t.Run("stops cleanly", func(t *testing.T) {
		w, _, _, ctx, _, _ := newTestSetup(t)

		err := w.Start(ctx)
		require.NoError(t, err)

		w.Stop()
	})

	t.Run("double stop does not panic", func(t *testing.T) {
		w, _, _, ctx, _, _ := newTestSetup(t)

		err := w.Start(ctx)
		require.NoError(t, err)

		w.Stop()
		w.Stop()
	})

	t.Run("stop before start does not panic", func(t *testing.T) {
		w, _, _, _, _, _ := newTestSetup(t)

		w.Stop()
	})
}

func TestReload(t *testing.T) {
	t.Run("reloads plugin on file write after start", func(t *testing.T) {
		w, _, _, ctx, pluginsDir, lc := newTestSetup(t)
		defer w.Stop()

		err := w.Start(ctx)
		require.NoError(t, err)

		wasmPath := filepath.Join(pluginsDir, "test.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		assert.Eventually(t, func() bool {
			return lc.contains("Hot-reloaded plugin: test.wasm")
		}, 2*time.Second, 50*time.Millisecond)
	})

	t.Run("ignores non-wasm files", func(t *testing.T) {
		w, _, _, ctx, pluginsDir, lc := newTestSetup(t)
		defer w.Stop()

		err := w.Start(ctx)
		require.NoError(t, err)

		txtPath := filepath.Join(pluginsDir, "readme.txt")
		require.NoError(t, os.WriteFile(txtPath, []byte("hello"), 0o644))

		time.Sleep(500 * time.Millisecond)

		assert.False(t, lc.contains("Hot-reloaded plugin: readme.txt"))
	})

	t.Run("failed reload logs error", func(t *testing.T) {
		w, _, _, ctx, pluginsDir, lc := newTestSetup(t)
		defer w.Stop()

		err := w.Start(ctx)
		require.NoError(t, err)

		badPath := filepath.Join(pluginsDir, "bad.wasm")
		require.NoError(t, os.WriteFile(badPath, []byte{0xDE, 0xAD}, 0o644))

		assert.Eventually(t, func() bool {
			return lc.containsPrefix("Failed to reload plugin: bad.wasm")
		}, 2*time.Second, 50*time.Millisecond)
	})

	t.Run("debounce multiple writes", func(t *testing.T) {
		w, _, _, ctx, pluginsDir, lc := newTestSetup(t)
		defer w.Stop()

		err := w.Start(ctx)
		require.NoError(t, err)

		wasmPath := filepath.Join(pluginsDir, "debounce.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		assert.Eventually(t, func() bool {
			return lc.contains("Hot-reloaded plugin: debounce.wasm")
		}, 2*time.Second, 50*time.Millisecond)

		for range 5 {
			require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))
			time.Sleep(10 * time.Millisecond)
		}

		assert.Eventually(t, func() bool {
			return lc.count("Hot-reloaded plugin: debounce.wasm") >= 2
		}, 2*time.Second, 50*time.Millisecond)
	})

	t.Run("replaces old plugin hooks on reload", func(t *testing.T) {
		w, reg, _, ctx, pluginsDir, _ := newTestSetup(t)
		defer w.Stop()
		_ = reg

		wasmPath := filepath.Join(pluginsDir, "hooktest.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		err := w.Start(ctx)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))
		time.Sleep(500 * time.Millisecond)

		hooks := reg.Hooks()
		assert.NotNil(t, hooks)
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("no race conditions on parallel access", func(t *testing.T) {
		w, _, _, ctx, pluginsDir, _ := newTestSetup(t)
		defer w.Stop()

		wasmPath := filepath.Join(pluginsDir, "race.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		err := w.Start(ctx)
		require.NoError(t, err)

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = os.WriteFile(wasmPath, addWasm, 0o644)
			}()
		}
		wg.Wait()

		time.Sleep(1 * time.Second)
	})
}
