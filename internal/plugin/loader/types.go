package loader

import (
	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
	"github.com/tetratelabs/wazero/api"
)

// LoadedPlugin pairs one plugin's domain metadata with its live WASM
// instance. The domain record stays pure; the runtime handle lives here,
// where the loader, registry, and dev watcher can see it.
type LoadedPlugin struct {
	plugin.Plugin
	Module api.Module
}

type PluginLoadResult struct {
	Plugin   LoadedPlugin
	Manifest *capability.Manifest
	Err      error
}
