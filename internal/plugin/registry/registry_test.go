package registry_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	return registry.NewRegistry(nil)
}

func newTestRegistryWithLogger(t *testing.T) (*registry.Registry, *strings.Builder) {
	t.Helper()
	var sb strings.Builder
	reg := registry.NewRegistry(func(msg string) { sb.WriteString(msg + "\n") })
	return reg, &sb
}

func testHandler(prefix string) plugin.HookHandler {
	return func(_ context.Context, data []byte) ([]byte, error) {
		return append([]byte(prefix+":"), data...), nil
	}
}

func TestRegisterAddsHook(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   plugin.DefaultPriority,
		Handler:    testHandler("a"),
	})

	require.NoError(t, err)
	assert.True(t, reg.HasHook(plugin.HookBeforeSave))
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	reg := newTestRegistry(t)

	regA := plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("a"),
	}

	err := reg.Register(regA)
	require.NoError(t, err)

	err = reg.Register(regA)
	assert.ErrorIs(t, err, plugin.ErrHookAlreadyRegistered)
}

func TestExecuteSingleHook(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("modified"),
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("modified:data"), result)
}

func TestExecutePriorityOrder(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("a"),
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	hooks := reg.Hooks()
	require.Len(t, hooks[plugin.HookBeforeSave], 2)
	assert.Equal(t, "plugin-b", hooks[plugin.HookBeforeSave][0].PluginName)
	assert.Equal(t, "plugin-a", hooks[plugin.HookBeforeSave][1].PluginName)
}

func TestExecuteChainsData(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler:    testHandler("first"),
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("second"),
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("second:first:data"), result)
}

func TestExecuteNoHooksRegistered(t *testing.T) {
	reg := newTestRegistry(t)

	_, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	assert.ErrorIs(t, err, plugin.ErrHookNotFound)
}

func TestExecuteHandlerFailFast(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("hook error")
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	_, err = reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	assert.ErrorIs(t, err, plugin.ErrHookExecutionFailed)
}

func TestUnregister(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("a"),
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	reg.Unregister("plugin-a")

	assert.True(t, reg.HasHook(plugin.HookBeforeSave))
	hooks := reg.Hooks()
	require.Len(t, hooks[plugin.HookBeforeSave], 1)
	assert.Equal(t, "plugin-b", hooks[plugin.HookBeforeSave][0].PluginName)
}

func TestUnregisterDoesNotAffectOtherPlugins(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("a"),
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookAfterPublish,
		Priority:   10,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	reg.Unregister("plugin-a")

	assert.False(t, reg.HasHook(plugin.HookBeforeSave))
	assert.True(t, reg.HasHook(plugin.HookAfterPublish))
}

func TestHooksReturnsCopy(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("a"),
	})
	require.NoError(t, err)

	hooks1 := reg.Hooks()
	hooks2 := reg.Hooks()

	hooks1Slice := hooks1[plugin.HookBeforeSave]
	hooks2Slice := hooks2[plugin.HookBeforeSave]
	assert.NotEqual(
		t,
		&hooks1Slice,
		&hooks2Slice,
		"should return different slice instances",
	)
}

func TestHasHook(t *testing.T) {
	reg := newTestRegistry(t)

	assert.False(t, reg.HasHook(plugin.HookBeforeSave))

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("a"),
	})
	require.NoError(t, err)

	assert.True(t, reg.HasHook(plugin.HookBeforeSave))
	assert.False(t, reg.HasHook(plugin.HookAfterPublish))
}

func TestConcurrentAccess(t *testing.T) {
	reg := newTestRegistry(t)
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(2)

		go func(i int) {
			defer wg.Done()
			_ = reg.Register(plugin.HookRegistration{
				PluginName: fmt.Sprintf("plugin-%d", i),
				HookName:   plugin.HookBeforeSave,
				Priority:   i,
				Handler:    testHandler(fmt.Sprintf("h%d", i)),
			})
		}(i)

		go func() {
			defer wg.Done()
			_ = reg.HasHook(plugin.HookBeforeSave)
			_ = reg.Hooks()
		}()
	}

	wg.Wait()
}

func TestRegisterRejectsNilHandler(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    nil,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler cannot be nil")
}

func TestUnregisterAllowsReRegistration(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("a"),
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookAfterPublish,
		Priority:   10,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	reg.Unregister("plugin-a")

	assert.False(t, reg.HasHook(plugin.HookBeforeSave))
	assert.False(t, reg.HasHook(plugin.HookAfterPublish))

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler:    testHandler("re-registered"),
	})
	require.NoError(t, err)

	assert.True(t, reg.HasHook(plugin.HookBeforeSave))
}

func TestExecuteOriginalDataUnchangedAfterHook(t *testing.T) {
	reg := newTestRegistry(t)

	original := []byte("original data")

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("modified"),
	})
	require.NoError(t, err)

	_, err = reg.Execute(context.Background(), plugin.HookBeforeSave, original)
	require.NoError(t, err)

	assert.Equal(t, []byte("original data"), original)
}

func TestExecuteHandlerReceivesCopyNotOriginal(t *testing.T) {
	reg := newTestRegistry(t)

	var receivedPtr uintptr
	original := []byte("original data")

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler: func(_ context.Context, data []byte) ([]byte, error) {
			receivedPtr = uintptr(unsafe.Pointer(&data[0]))
			data[0] = 'X'
			return data, nil
		},
	})
	require.NoError(t, err)

	_, err = reg.Execute(context.Background(), plugin.HookBeforeSave, original)
	require.NoError(t, err)

	originalPtr := uintptr(unsafe.Pointer(&original[0]))
	assert.NotEqual(t, originalPtr, receivedPtr, "handler should receive a copy, not the original")
	assert.Equal(t, byte('o'), original[0], "original data must be unchanged")
}

func TestExecuteMutationDetectionLogsWarning(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler: func(_ context.Context, data []byte) ([]byte, error) {
			data[0] = 'M'
			return []byte("returned"), nil
		},
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("returned"), result)
	assert.Contains(t, log.String(), "Plugin attempted to modify immutable data")
}

func TestExecuteMutationIgnored(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler: func(_ context.Context, data []byte) ([]byte, error) {
			data[0] = 'X'
			return nil, nil
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	_, err = reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Contains(t, log.String(), "Plugin attempted to modify immutable data")
	assert.Contains(t, log.String(), "Hook did not return data, using original")
}

func TestExecuteSequentialChaining(t *testing.T) {
	reg := newTestRegistry(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler:    testHandler("a"),
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("b:a:data"), result)
}

func TestExecuteNoReturnFallbackNil(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, nil
		},
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), result)
	assert.Contains(t, log.String(), "Hook did not return data, using original")
}

func TestExecuteNoReturnFallbackEmpty(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "test-plugin",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return []byte{}, nil
		},
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), result)
	assert.Contains(t, log.String(), "Hook did not return data, using original")
}

func TestExecuteFallbackDataPassedToNextHandler(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, nil
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler:    testHandler("b"),
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("b:data"), result)
	assert.Contains(t, log.String(), "Hook did not return data, using original")
}

func TestExecuteFailFastReturnsError(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	secondCalled := false
	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.FailFast,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-b",
		HookName:    plugin.HookBeforeSave,
		Priority:    10,
		FailureMode: plugin.FailFast,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			secondCalled = true
			return []byte("b"), nil
		},
	})
	require.NoError(t, err)

	_, err = reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	assert.ErrorIs(t, err, plugin.ErrHookExecutionFailed)
	assert.Contains(t, log.String(), "Hook execution failed")
	assert.False(t, secondCalled, "second handler must not be called after FailFast error")
	_ = secondCalled
}

func TestExecuteFailFastLogsError(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.FailFast,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("something went wrong")
		},
	})
	require.NoError(t, err)

	_, err = reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	assert.ErrorIs(t, err, plugin.ErrHookExecutionFailed)
	assert.Contains(t, log.String(), "Hook execution failed: something went wrong")
}

func TestExecuteLogAndContinueContinuesAfterError(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.LogAndContinue,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-b",
		HookName:    plugin.HookBeforeSave,
		Priority:    10,
		FailureMode: plugin.LogAndContinue,
		Handler:     testHandler("b"),
	})
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("b:data"), result)
	assert.Contains(t, log.String(), "Hook execution failed")
}

func TestExecuteLogAndContinuePassesDataBeforeFailedHandler(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.LogAndContinue,
		Handler:     testHandler("first"),
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-b",
		HookName:    plugin.HookBeforeSave,
		Priority:    10,
		FailureMode: plugin.LogAndContinue,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-c",
		HookName:    plugin.HookBeforeSave,
		Priority:    15,
		FailureMode: plugin.LogAndContinue,
		Handler:     testHandler("third"),
	})
	require.NoError(t, err)

	result, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, execErr)
	assert.Equal(t, []byte("third:first:data"), result, "third handler should receive data from before the failed handler")
	assert.Contains(t, log.String(), "Hook execution failed")
}

func TestExecuteLogAndContinueReturnsNilError(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.LogAndContinue,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
	})
	require.NoError(t, err)

	_, err = reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	_ = log
	require.NoError(t, err, "LogAndContinue should return nil error even after handler failure")
}

func TestExecuteFallbackCallsFallbackHandler(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	fallbackCalled := false
	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.Fallback,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
		FallbackHandler: func(_ context.Context, data []byte) ([]byte, error) {
			fallbackCalled = true
			return append([]byte("fallback: "), data...), nil
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-b",
		HookName:    plugin.HookBeforeSave,
		Priority:    10,
		FailureMode: plugin.FailFast,
		Handler:     testHandler("b"),
	})
	require.NoError(t, err)

	result, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, execErr)
	assert.True(t, fallbackCalled, "fallback handler must be called when primary handler fails")
	assert.Equal(t, []byte("b:fallback: data"), result)
	assert.Contains(t, log.String(), "Hook failed, using fallback")
}

func TestExecuteFallbackResultPassedToNextHandler(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.Fallback,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
		FallbackHandler: func(_ context.Context, data []byte) ([]byte, error) {
			return append([]byte("fb: "), data...), nil
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-b",
		HookName:    plugin.HookBeforeSave,
		Priority:    10,
		FailureMode: plugin.FailFast,
		Handler:     testHandler("b"),
	})
	require.NoError(t, err)

	result, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, execErr)
	assert.Equal(t, []byte("b:fb: data"), result, "fallback result should be passed to next handler")
	_ = log
}

func TestExecuteFallbackNilHandlerDegradesToLogAndContinue(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	secondCalled := false
	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.Fallback,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
		FallbackHandler: nil,
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-b",
		HookName:    plugin.HookBeforeSave,
		Priority:    10,
		FailureMode: plugin.FailFast,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			secondCalled = true
			return append([]byte("b: "), []byte("data")...), nil
		},
	})
	require.NoError(t, err)

	result, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, execErr)
	assert.True(t, secondCalled, "next handler should be called when fallback is nil (degrades to LogAndContinue)")
	assert.Equal(t, []byte("b: data"), result)
	assert.Contains(t, log.String(), "Hook execution failed")
}

func TestExecuteDefaultFailureModeIsFailFast(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	secondCalled := false
	err := reg.Register(plugin.HookRegistration{
		PluginName: "plugin-a",
		HookName:   plugin.HookBeforeSave,
		Priority:   5,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName: "plugin-b",
		HookName:   plugin.HookBeforeSave,
		Priority:   10,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			secondCalled = true
			return []byte("b"), nil
		},
	})
	require.NoError(t, err)

	_, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	assert.ErrorIs(t, execErr, plugin.ErrHookExecutionFailed)
	assert.False(t, secondCalled, "second handler must not be called when default (FailFast) applies")
	_ = log
}

func TestExecuteFallbackLogMessage(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.Fallback,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
		FallbackHandler: func(_ context.Context, data []byte) ([]byte, error) {
			return data, nil
		},
	})
	require.NoError(t, err)

	_, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, execErr)
	assert.Contains(t, log.String(), "Hook failed, using fallback")
}

func TestExecuteFallbackHandlerErrorLogsFallbackError(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.Fallback,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("primary error")
		},
		FallbackHandler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("fallback error")
		},
	})
	require.NoError(t, err)

	result, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	require.NoError(t, execErr)
	assert.NotContains(t, log.String(), "primary error", "should log fallback error, not primary error")
	assert.Contains(t, log.String(), "fallback error")
	assert.Equal(t, []byte("data"), result, "should use original data when both primary and fallback fail")
}

func TestExecuteUnrecognizedFailureModeTreatedAsFailFast(t *testing.T) {
	reg, log := newTestRegistryWithLogger(t)

	secondCalled := false
	err := reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-a",
		HookName:    plugin.HookBeforeSave,
		Priority:    5,
		FailureMode: plugin.FailureMode(99),
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("handler error")
		},
	})
	require.NoError(t, err)

	err = reg.Register(plugin.HookRegistration{
		PluginName:  "plugin-b",
		HookName:    plugin.HookBeforeSave,
		Priority:    10,
		Handler: func(_ context.Context, _ []byte) ([]byte, error) {
			secondCalled = true
			return []byte("b"), nil
		},
	})
	require.NoError(t, err)

	_, execErr := reg.Execute(context.Background(), plugin.HookBeforeSave, []byte("data"))
	assert.ErrorIs(t, execErr, plugin.ErrHookExecutionFailed)
	assert.False(t, secondCalled, "unrecognized failure mode should be treated as FailFast")
	assert.Contains(t, log.String(), "Hook execution failed")
}
