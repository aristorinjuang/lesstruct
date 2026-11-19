package blog

import (
	"context"
	"sort"

	"github.com/aristorinjuang/lesstruct/internal/app"
	plugingenapi "github.com/aristorinjuang/lesstruct/modules/plugingen/api"
)

// pluginStatusReader is the edition-side PluginStatusReader seam over the
// composition root's plugin system: installed .wasm names come from the
// loaded-plugin metadata, hook lists from the live registry grouped by
// plugin. A plugin with registered hooks reports loaded; a loaded plugin
// with no hooks (or an unknown source) reports installed. The registry is
// authoritative, so DEV_MODE hot reloads reflect immediately. A nil source
// (tests, lean binaries) reads as an empty list.
type pluginStatusReader struct {
	source app.PluginSystemSnapshot
}

func (r pluginStatusReader) LoadedPlugins(_ context.Context) []plugingenapi.PluginStatus {
	if r.source == nil {
		return []plugingenapi.PluginStatus{}
	}
	hooked := make(map[string]map[string]struct{})
	for hook, registrations := range r.source.Hooks() {
		for _, registration := range registrations {
			if registration.PluginName == "" {
				continue
			}
			set, ok := hooked[registration.PluginName]
			if !ok {
				set = make(map[string]struct{})
				hooked[registration.PluginName] = set
			}
			set[string(hook)] = struct{}{}
		}
	}
	plugins := r.source.Plugins()
	result := make([]plugingenapi.PluginStatus, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.Name == "" {
			continue
		}
		hooks := make([]string, 0, len(hooked[plugin.Name]))
		for hook := range hooked[plugin.Name] {
			hooks = append(hooks, hook)
		}
		sort.Strings(hooks)
		status := "installed"
		if len(hooks) > 0 {
			status = "loaded"
		}
		result = append(result, plugingenapi.PluginStatus{
			Name:   plugin.Name,
			Status: status,
			Hooks:  hooks,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// NewPluginStatusReader builds the edition's plugin status seam over the
// composition root's plugin system snapshot.
func NewPluginStatusReader(source app.PluginSystemSnapshot) plugingenapi.PluginStatusReader {
	return pluginStatusReader{source: source}
}
