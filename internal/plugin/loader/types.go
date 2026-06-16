package loader

import (
	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
)

type PluginLoadResult struct {
	Plugin   plugin.Plugin
	Manifest *capability.Manifest
	Err      error
}
