package loader_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/plugin/loader"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationLoadAllWithValidWasm(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "adder.wasm"),
		addWasm,
		0o644,
	))

	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	l := loader.NewLoader(rt, pluginsDir, nil, func(string) {})

	loaded, results := l.LoadAll(ctx)

	require.Len(t, loaded, 1)
	assert.Equal(t, "adder", loaded[0].Name)
	assert.Equal(t, "loaded", loaded[0].Status.String())
	assert.NotNil(t, loaded[0].Module)
	assert.Len(t, results, 1)
	assert.Nil(t, results[0].Err)
	assert.Nil(t, results[0].Manifest)

	fn := loaded[0].Module.ExportedFunction("add")
	require.NotNil(t, fn)

	res, err := fn.Call(ctx, 3, 4)
	require.NoError(t, err)
	assert.EqualValues(t, 7, res[0])
}

func TestIntegrationLoadAllWithCorruptedWasm(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "corrupted.wasm"),
		[]byte{0x00, 0x01, 0x02, 0x03},
		0o644,
	))

	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	l := loader.NewLoader(rt, pluginsDir, nil, func(string) {})

	loaded, results := l.LoadAll(ctx)

	assert.Empty(t, loaded)
	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Err)
	assert.Contains(t, results[0].Err.Error(), "invalid WASM format")
}

func TestIntegrationMixedValidAndCorruptedPlugins(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "good.wasm"),
		addWasm,
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "broken.wasm"),
		[]byte{0xFF, 0xFF, 0xFF, 0xFF},
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "another-good.wasm"),
		addWasm,
		0o644,
	))

	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	l := loader.NewLoader(rt, pluginsDir, nil, func(string) {})

	loaded, results := l.LoadAll(ctx)

	require.Len(t, loaded, 2)
	names := map[string]bool{loaded[0].Name: true, loaded[1].Name: true}
	assert.True(t, names["good"])
	assert.True(t, names["another-good"])
	assert.Len(t, results, 3)
}

func TestIntegrationLoadPluginWithManifest(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "enriched.wasm"),
		addWasm,
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "enriched.manifest"),
		[]byte(`
name = "enriched"
version = "1.0.0"

[capabilities]
http = ["https://api.example.com/*"]
`),
		0o644,
	))

	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	l := loader.NewLoader(rt, pluginsDir, nil, func(string) {})

	loaded, results := l.LoadAll(ctx)

	require.Len(t, loaded, 1)
	assert.Equal(t, "enriched", loaded[0].Name)
	assert.Len(t, results, 1)
	assert.Nil(t, results[0].Err)
	assert.NotNil(t, results[0].Manifest)
	assert.True(t, results[0].Manifest.HasHTTP())
}

func TestIntegrationLoadPluginWithInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "bad-manifest.wasm"),
		addWasm,
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(pluginsDir, "bad-manifest.manifest"),
		[]byte(`name = ""`),
		0o644,
	))

	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })

	l := loader.NewLoader(rt, pluginsDir, nil, func(string) {})

	loaded, results := l.LoadAll(ctx)

	assert.Empty(t, loaded)
	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Err)
	assert.Contains(t, results[0].Err.Error(), "manifest")
}