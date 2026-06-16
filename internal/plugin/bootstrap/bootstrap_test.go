package bootstrap_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/plugin/bootstrap"
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
	return slices.Contains(l.messages, s)
}

func (l *logCapture) containsAny(subs ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.messages {
		for _, sub := range subs {
			if strings.Contains(m, sub) {
				return true
			}
		}
	}
	return false
}

func setupPluginsDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "plugins")
}

func TestNewPluginSystem(t *testing.T) {
	t.Run("production mode initialization", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			false,
			nil,
			lc.log,
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ps.Close(context.Background()) })

		assert.True(t, lc.contains("Production mode activated - startup-load enabled"))
		assert.NotNil(t, ps.Registry())
		assert.Empty(t, ps.Plugins())
	})

	t.Run("development mode initialization", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			true,
			nil,
			lc.log,
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ps.Close(context.Background()) })

		assert.True(t, lc.contains("Development mode activated - hot-reload enabled"))
		assert.NotNil(t, ps.Registry())
	})

	t.Run("plugin load failure logs error and continues", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		badPath := filepath.Join(pluginsDir, "broken.wasm")
		require.NoError(t, os.WriteFile(badPath, []byte{0xDE, 0xAD}, 0o644))

		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			false,
			nil,
			lc.log,
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ps.Close(context.Background()) })

		assert.True(t, lc.containsAny("Failed to load plugin: broken.wasm"))
		assert.True(t, lc.contains("Production mode activated - startup-load enabled"))
	})

	t.Run("loads valid plugin and discovers hooks", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		wasmPath := filepath.Join(pluginsDir, "myplugin.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			false,
			nil,
			lc.log,
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ps.Close(context.Background()) })

		assert.Len(t, ps.Plugins(), 1)
		assert.Equal(t, "myplugin", ps.Plugins()[0].Name)
	})

	t.Run("empty plugins directory succeeds", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			false,
			nil,
			lc.log,
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ps.Close(context.Background()) })

		assert.Empty(t, ps.Plugins())
		assert.True(t, lc.contains("Production mode activated - startup-load enabled"))
	})

	t.Run("nil logger uses no-op", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			false,
			nil,
			nil,
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ps.Close(context.Background()) })

		assert.NotNil(t, ps)
	})
}

func TestClose(t *testing.T) {
	t.Run("cleans up runtime and unloads plugins", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

		wasmPath := filepath.Join(pluginsDir, "cleanup.wasm")
		require.NoError(t, os.WriteFile(wasmPath, addWasm, 0o644))

		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			false,
			nil,
			lc.log,
		)
		require.NoError(t, err)

		err = ps.Close(context.Background())
		require.NoError(t, err)
	})

	t.Run("stops watcher in development mode", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			true,
			nil,
			lc.log,
		)
		require.NoError(t, err)

		err = ps.Close(context.Background())
		require.NoError(t, err)
	})

	t.Run("double close does not panic", func(t *testing.T) {
		pluginsDir := setupPluginsDir(t)
		lc := &logCapture{}

		ps, err := bootstrap.NewPluginSystem(
			context.Background(),
			pluginsDir,
			false,
			nil,
			lc.log,
		)
		require.NoError(t, err)

		err = ps.Close(context.Background())
		require.NoError(t, err)

		err = ps.Close(context.Background())
		require.NoError(t, err)
	})
}