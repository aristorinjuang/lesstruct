package plugin_test

import (
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
)

func TestPluginStatusConstants(t *testing.T) {
	assert.Equal(t, plugin.Status("loaded"), plugin.StatusLoaded)
	assert.Equal(t, plugin.Status("failed"), plugin.StatusFailed)
	assert.Equal(t, plugin.Status("unloaded"), plugin.StatusUnloaded)
}

func TestPluginStruct(t *testing.T) {
	now := time.Now()

	p := plugin.Plugin{
		Name:     "test-plugin",
		FilePath: "/plugins/test-plugin.wasm",
		Status:   plugin.StatusLoaded,
		LoadedAt: now,
	}

	assert.Equal(t, "test-plugin", p.Name)
	assert.Equal(t, "/plugins/test-plugin.wasm", p.FilePath)
	assert.Equal(t, plugin.StatusLoaded, p.Status)
	assert.Equal(t, now, p.LoadedAt)
}

func TestPluginStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   plugin.Status
		expected string
	}{
		{"loaded", plugin.StatusLoaded, "loaded"},
		{"failed", plugin.StatusFailed, "failed"},
		{"unloaded", plugin.StatusUnloaded, "unloaded"},
		{"unknown", plugin.Status("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Run("ErrPluginNotFound", func(t *testing.T) {
		assert.Equal(t, "plugin not found", plugin.ErrPluginNotFound.Error())
	})

	t.Run("ErrPluginInvalidFormat", func(t *testing.T) {
		assert.Equal(t, "invalid WASM format", plugin.ErrPluginInvalidFormat.Error())
	})

	t.Run("ErrPluginLoadFailed", func(t *testing.T) {
		assert.Equal(t, "plugin load failed", plugin.ErrPluginLoadFailed.Error())
	})

	t.Run("errors are distinct", func(t *testing.T) {
		assert.NotEqual(t, plugin.ErrPluginNotFound, plugin.ErrPluginInvalidFormat)
		assert.NotEqual(t, plugin.ErrPluginNotFound, plugin.ErrPluginLoadFailed)
		assert.NotEqual(t, plugin.ErrPluginInvalidFormat, plugin.ErrPluginLoadFailed)
	})
}
