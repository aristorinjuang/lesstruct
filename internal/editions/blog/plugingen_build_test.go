package blog

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/hostfunctions"
	"github.com/aristorinjuang/lesstruct/internal/plugin/loader"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	plugingeninfra "github.com/aristorinjuang/lesstruct/modules/plugingen/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlugingenBuildAndLoad compiles a real fixture with the system TinyGo,
// installs it, and loads it through the host loader plus hook discovery —
// the full production path a generated plugin travels. It skips when no
// TinyGo is installed (CI stays hermetic) and when the installed TinyGo
// predates the system Go release.
func TestPlugingenBuildAndLoad(t *testing.T) {
	project := plugingendomain.NewProject(
		"tinygo-loader-e2e",
		"go",
		[]string{"before_save"},
		[]string{"log_info"},
		map[string]string{
			"main.go": "package main\n\nimport \"unsafe\"\n\nvar resultBuf [4096]byte\n\n//export hook_before_save\nfunc hookBeforeSave(offset uint32, length uint32) uint32 {\n\tinput := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)\n\tn := copy(resultBuf[:], input)\n\t_ = n\n\treturn uint32(uintptr(unsafe.Pointer(&resultBuf[0])))\n}\n\n//export __hook_result_len\nfunc hookResultLen(inputLen uint32) uint32 { return uint32(len(resultBuf)) }\n\nfunc main() {}\n",
			"go.mod": "module tinygo-loader-e2e\n\ngo 1.23\n",
		},
		"",
	)
	pluginsDir := t.TempDir()
	builder := plugingeninfra.NewTinyGoBuilder("", pluginsDir, 4*time.Minute, true, nil)
	result, err := builder.Build(context.Background(), project)
	if err != nil && strings.Contains(err.Error(), "requires go version") {
		t.Skipf("system tinygo predates system go: %v", err)
	}
	require.NoError(t, err)
	assert.True(t, result.Installed)

	ctx := context.Background()
	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(ctx) })
	hostFuncs := hostfunctions.NewRegistry(nil, nil, func(string) {})
	ldr := loader.NewLoader(rt, pluginsDir, hostFuncs, func(string) {})
	require.NoError(t, ldr.EnsurePluginsDir())

	loaded, manifest, err := ldr.Load(ctx, filepath.Join(pluginsDir, "tinygo-loader-e2e.wasm"))
	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, "tinygo-loader-e2e", manifest.Name)
	assert.NotNil(t, loaded.Module)
	require.NotNil(t, loaded.Module.Memory())

	reg := registry.NewRegistry(func(string) {})
	require.NoError(t, registry.DiscoverHooks(ctx, loaded, reg, rt, func(string) {}))
	assert.True(t, reg.HasHook(plugin.HookBeforeSave))
	require.NoError(t, loaded.Module.Close(ctx))
}
