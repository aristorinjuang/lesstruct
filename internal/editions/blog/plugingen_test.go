package blog

import (
	"context"
	"strings"
	"testing"

	plugindomain "github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePluginSystem struct {
	plugins []plugindomain.Plugin
	hooks   map[plugindomain.HookName][]plugindomain.HookRegistration
}

func (f fakePluginSystem) Plugins() []plugindomain.Plugin {
	return f.plugins
}

func (f fakePluginSystem) Hooks() map[plugindomain.HookName][]plugindomain.HookRegistration {
	return f.hooks
}

func TestPluginStatusReader(t *testing.T) {
	tests := []struct {
		name     string
		source   fakePluginSystem
		expected []string
		wantErr  bool
	}{
		{
			name: "success - loaded plugin with hooks",
			source: fakePluginSystem{
				plugins: []plugindomain.Plugin{{Name: "hello-world", Status: plugindomain.StatusLoaded}},
				hooks: map[plugindomain.HookName][]plugindomain.HookRegistration{
					plugindomain.HookBeforeSave: {{PluginName: "hello-world", HookName: plugindomain.HookBeforeSave}},
				},
			},
			expected: []string{"hello-world:loaded:BeforeSaveContent"},
			wantErr:  false,
		},
		{
			name: "success - multiple hooks union and sort",
			source: fakePluginSystem{
				plugins: []plugindomain.Plugin{{Name: "rich-plugin", Status: plugindomain.StatusLoaded}},
				hooks: map[plugindomain.HookName][]plugindomain.HookRegistration{
					plugindomain.HookAfterPublish: {{PluginName: "rich-plugin", HookName: plugindomain.HookAfterPublish}},
					plugindomain.HookBeforeSave:   {{PluginName: "rich-plugin", HookName: plugindomain.HookBeforeSave}},
					plugindomain.HookAfterCreate:  {{PluginName: "rich-plugin", HookName: plugindomain.HookAfterCreate}},
				},
			},
			expected: []string{"rich-plugin:loaded:AfterCreateContent,AfterPublishContent,BeforeSaveContent"},
			wantErr:  false,
		},
		{
			name: "success - orphan registration without plugin is not emitted",
			source: fakePluginSystem{
				plugins: []plugindomain.Plugin{{Name: "hello-world", Status: plugindomain.StatusLoaded}},
				hooks: map[plugindomain.HookName][]plugindomain.HookRegistration{
					plugindomain.HookBeforeSave:   {{PluginName: "hello-world", HookName: plugindomain.HookBeforeSave}},
					plugindomain.HookBeforeDelete: {{PluginName: "ghost-plugin", HookName: plugindomain.HookBeforeDelete}},
				},
			},
			expected: []string{"hello-world:loaded:BeforeSaveContent"},
			wantErr:  false,
		},
		{
			name: "success - loaded plugin without hooks reports installed",
			source: fakePluginSystem{
				plugins: []plugindomain.Plugin{{Name: "quiet-one", Status: plugindomain.StatusLoaded}},
				hooks:   map[plugindomain.HookName][]plugindomain.HookRegistration{},
			},
			expected: []string{"quiet-one:installed:"},
			wantErr:  false,
		},
		{
			name:     "success - empty source",
			source:   fakePluginSystem{},
			expected: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewPluginStatusReader(tt.source)
			plugins := reader.LoadedPlugins(context.Background())
			got := make([]string, 0, len(plugins))
			for _, plugin := range plugins {
				var hooks strings.Builder
				for i, hook := range plugin.Hooks {
					if i > 0 {
						hooks.WriteString(",")
					}
					hooks.WriteString(hook)
				}
				got = append(got, plugin.Name+":"+plugin.Status+":"+hooks.String())
			}
			assert.Equal(t, tt.expected, got)
			require.Equal(t, false, tt.wantErr)
		})
	}
}

func TestPluginStatusReaderNilSource(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - nil source reads empty",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewPluginStatusReader(nil)
			assert.Empty(t, reader.LoadedPlugins(context.Background()))
			require.Equal(t, false, tt.wantErr)
		})
	}
}
