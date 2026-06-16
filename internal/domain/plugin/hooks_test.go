package plugin_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
)

func TestHookNameConstants(t *testing.T) {
	tests := []struct {
		name     string
		hook     plugin.HookName
		expected string
	}{
		{
			name:     "HookBeforeSave",
			hook:     plugin.HookBeforeSave,
			expected: "BeforeSaveContent",
		},
		{
			name:     "HookAfterPublish",
			hook:     plugin.HookAfterPublish,
			expected: "AfterPublishContent",
		},
		{
			name:     "HookBeforeDelete",
			hook:     plugin.HookBeforeDelete,
			expected: "BeforeDeleteContent",
		},
		{
			name:     "HookAfterCreate",
			hook:     plugin.HookAfterCreate,
			expected: "AfterCreateContent",
		},
		{
			name:     "HookOnPluginLoaded",
			hook:     plugin.HookOnPluginLoaded,
			expected: "OnPluginLoaded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.hook))
		})
	}
}

func TestDefaultPriority(t *testing.T) {
	assert.Equal(t, 100, plugin.DefaultPriority)
}

func TestHookHandler(t *testing.T) {
	var handler plugin.HookHandler = func(_ context.Context, data []byte) ([]byte, error) {
		return append([]byte("modified: "), data...), nil
	}

	result, err := handler(context.Background(), []byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("modified: test"), result)
}

func TestHookRegistration(t *testing.T) {
	reg := plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler: func(_ context.Context, data []byte) ([]byte, error) {
			return data, nil
		},
	}

	assert.Equal(t, "test-plugin", reg.PluginName)
	assert.Equal(t, plugin.HookBeforeSave, reg.HookName)
	assert.Equal(t, 10, reg.Priority)
	assert.NotNil(t, reg.Handler)
}

func TestHookSentinelErrors(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  string
	}{
		{
			name: "ErrHookNotFound",
			err:  plugin.ErrHookNotFound,
			want: "hook not found",
		},
		{
			name: "ErrHookExecutionFailed",
			err:  plugin.ErrHookExecutionFailed,
			want: "hook execution failed",
		},
		{
			name: "ErrHookAlreadyRegistered",
			err:  plugin.ErrHookAlreadyRegistered,
			want: "hook already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualError(t, tt.err, tt.want)
		})
	}
}

func TestHookNameString(t *testing.T) {
	h := plugin.HookName("custom_hook")
	assert.Equal(t, "custom_hook", h.String())
}

func TestResolveWasmHookName(t *testing.T) {
	tests := []struct {
		name     string
		export   string
		expected plugin.HookName
	}{
		{name: "before_save maps to HookBeforeSave", export: "before_save", expected: plugin.HookBeforeSave},
		{name: "after_publish maps to HookAfterPublish", export: "after_publish", expected: plugin.HookAfterPublish},
		{name: "before_delete maps to HookBeforeDelete", export: "before_delete", expected: plugin.HookBeforeDelete},
		{name: "after_create maps to HookAfterCreate", export: "after_create", expected: plugin.HookAfterCreate},
		{name: "on_plugin_loaded maps to HookOnPluginLoaded", export: "on_plugin_loaded", expected: plugin.HookOnPluginLoaded},
		{name: "unknown export passes through", export: "custom_hook", expected: plugin.HookName("custom_hook")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, plugin.ResolveWasmHookName(tt.export))
		})
	}
}

func TestFailureModeString(t *testing.T) {
	tests := []struct {
		name     string
		mode     plugin.FailureMode
		expected string
	}{
		{name: "FailFast", mode: plugin.FailFast, expected: "fail-fast"},
		{name: "LogAndContinue", mode: plugin.LogAndContinue, expected: "log-and-continue"},
		{name: "Fallback", mode: plugin.Fallback, expected: "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestParseFailureMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected plugin.FailureMode
	}{
		{name: "fail-fast", input: "fail-fast", expected: plugin.FailFast},
		{name: "log-and-continue", input: "log-and-continue", expected: plugin.LogAndContinue},
		{name: "fallback", input: "fallback", expected: plugin.Fallback},
		{name: "unknown returns FailFast", input: "unknown-mode", expected: plugin.FailFast},
		{name: "empty returns FailFast", input: "", expected: plugin.FailFast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, plugin.ParseFailureMode(tt.input))
		})
	}
}

func TestDefaultFailureMode(t *testing.T) {
	assert.Equal(t, plugin.FailFast, plugin.DefaultFailureMode)
}

func TestHookRegistrationWithFailureMode(t *testing.T) {
	reg := plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler: func(_ context.Context, data []byte) ([]byte, error) {
			return data, nil
		},
		FailureMode: plugin.LogAndContinue,
	}

	assert.Equal(t, plugin.LogAndContinue, reg.FailureMode)
}

func TestHookRegistrationWithFallbackHandler(t *testing.T) {
	fallback := func(_ context.Context, data []byte) ([]byte, error) {
		return append([]byte("fallback: "), data...), nil
	}

	reg := plugin.HookRegistration{
		PluginName:     "test-plugin",
		HookName:       plugin.HookBeforeSave,
		Priority:       10,
		Handler:        func(_ context.Context, data []byte) ([]byte, error) { return data, nil },
		FailureMode:    plugin.Fallback,
		FallbackHandler: fallback,
	}

	assert.Equal(t, plugin.Fallback, reg.FailureMode)
	assert.NotNil(t, reg.FallbackHandler)
}
