package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pluginadapter "github.com/aristorinjuang/lesstruct/internal/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookExecutorAdapter(t *testing.T) {
	t.Run("execute with nil registry returns nil without error", func(t *testing.T) {
		adapter := pluginadapter.NewHookExecutorAdapter(nil)
		result, err := adapter.Execute(context.Background(), plugin.HookBeforeSave, []byte(`{}`))

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("execute with no hooks registered returns nil without error", func(t *testing.T) {
		reg := registry.NewRegistry(func(string) {})
		adapter := pluginadapter.NewHookExecutorAdapter(reg)
		result, err := adapter.Execute(context.Background(), plugin.HookBeforeSave, []byte(`{"title":"test"}`))

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("execute with registered hook transforms data", func(t *testing.T) {
		reg := registry.NewRegistry(func(string) {})
		err := reg.Register(plugin.HookRegistration{
			PluginName:  "test-plugin",
			HookName:    plugin.HookBeforeSave,
			Priority:    10,
			FailureMode: plugin.FailFast,
			Handler: func(_ context.Context, data []byte) ([]byte, error) {
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				m["modified_by"] = "test-plugin"
				return json.Marshal(m)
			},
		})
		require.NoError(t, err)

		adapter := pluginadapter.NewHookExecutorAdapter(reg)
		input := []byte(`{"title":"test"}`)
		result, err := adapter.Execute(context.Background(), plugin.HookBeforeSave, input)

		require.NoError(t, err)
		require.NotNil(t, result)

		var parsed map[string]any
		_ = json.Unmarshal(result, &parsed)
		assert.Equal(t, "test-plugin", parsed["modified_by"])
		assert.Equal(t, "test", parsed["title"])
	})

	t.Run("execute with hook returning nil data returns original input", func(t *testing.T) {
		reg := registry.NewRegistry(func(string) {})
		err := reg.Register(plugin.HookRegistration{
			PluginName:  "test-plugin",
			HookName:    plugin.HookAfterCreate,
			Priority:    10,
			FailureMode: plugin.FailFast,
			Handler: func(_ context.Context, data []byte) ([]byte, error) {
				return nil, nil
			},
		})
		require.NoError(t, err)

		adapter := pluginadapter.NewHookExecutorAdapter(reg)
		input := []byte(`{"title":"test"}`)
		result, err := adapter.Execute(context.Background(), plugin.HookAfterCreate, input)

		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("execute with failing hook returns error", func(t *testing.T) {
		reg := registry.NewRegistry(func(string) {})
		err := reg.Register(plugin.HookRegistration{
			PluginName:  "test-plugin",
			HookName:    plugin.HookBeforeSave,
			Priority:    10,
			FailureMode: plugin.FailFast,
			Handler: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, errors.New("hook execution error")
			},
		})
		require.NoError(t, err)

		adapter := pluginadapter.NewHookExecutorAdapter(reg)
		result, err := adapter.Execute(context.Background(), plugin.HookBeforeSave, []byte(`{}`))

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "hook execution error")
	})

	t.Run("execute with LogAndContinue failing hook returns last successful result", func(t *testing.T) {
		reg := registry.NewRegistry(func(string) {})

		err := reg.Register(plugin.HookRegistration{
			PluginName:  "failing-plugin",
			HookName:    plugin.HookBeforeSave,
			Priority:    10,
			FailureMode: plugin.LogAndContinue,
			Handler: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, errors.New("plugin failed")
			},
		})
		require.NoError(t, err)

		err = reg.Register(plugin.HookRegistration{
			PluginName:  "working-plugin",
			HookName:    plugin.HookBeforeSave,
			Priority:    20,
			FailureMode: plugin.FailFast,
			Handler: func(_ context.Context, data []byte) ([]byte, error) {
				var m map[string]any
				_ = json.Unmarshal(data, &m)
				m["processed"] = true
				return json.Marshal(m)
			},
		})
		require.NoError(t, err)

		adapter := pluginadapter.NewHookExecutorAdapter(reg)
		result, err := adapter.Execute(context.Background(), plugin.HookBeforeSave, []byte(`{"title":"test"}`))

		require.NoError(t, err)
		require.NotNil(t, result)

		var parsed map[string]any
		_ = json.Unmarshal(result, &parsed)
		assert.Equal(t, true, parsed["processed"])
	})

	t.Run("execute with unregistered hook name returns nil", func(t *testing.T) {
		reg := registry.NewRegistry(func(string) {})
		err := reg.Register(plugin.HookRegistration{
			PluginName:  "test-plugin",
			HookName:    plugin.HookBeforeSave,
			Priority:    10,
			FailureMode: plugin.FailFast,
			Handler: func(_ context.Context, data []byte) ([]byte, error) {
				return data, nil
			},
		})
		require.NoError(t, err)

		adapter := pluginadapter.NewHookExecutorAdapter(reg)
		result, err := adapter.Execute(context.Background(), plugin.HookAfterPublish, []byte(`{}`))

		require.NoError(t, err)
		assert.Nil(t, result)
	})
}
