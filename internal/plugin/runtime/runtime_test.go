package runtime_test

import (
	"context"
	"strings"
	"testing"
	"time"

	runtime "github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuntime(t *testing.T) {
	t.Run("creates runtime without error", func(t *testing.T) {
		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })
		require.NotNil(t, rt)
	})

	t.Run("creates runtime with default config values", func(t *testing.T) {
		ctx := context.Background()
		cfg := runtime.RuntimeConfig{}
		rt, err := runtime.NewRuntime(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		assert.EqualValues(t, 64*1024*1024, rt.Config().MaxMemoryBytes)
		assert.Equal(t, runtime.DefaultMaxExecTime, rt.Config().MaxExecutionTime)
	})
}

func TestGetVersion(t *testing.T) {
	t.Run("returns non-empty version string", func(t *testing.T) {
		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		version := rt.GetVersion()
		assert.NotEmpty(t, version)
	})
}

func TestClose(t *testing.T) {
	t.Run("cleans up without error", func(t *testing.T) {
		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)

		err = rt.Close(ctx)
		assert.NoError(t, err)
	})
}

func TestDefaultMaxMemory(t *testing.T) {
	t.Run("memory limit constant is 64MB", func(t *testing.T) {
		assert.EqualValues(t, 64*1024*1024, runtime.DefaultMaxMemory)
	})
}

func TestDefaultMaxExecTime(t *testing.T) {
	t.Run("default execution time is 30 seconds", func(t *testing.T) {
		assert.Equal(t, 30*time.Second, runtime.DefaultMaxExecTime)
	})
}

func TestRuntimeConfigDefaults(t *testing.T) {
	t.Run("default config has standard memory limit", func(t *testing.T) {
		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		assert.EqualValues(t, runtime.DefaultMaxMemory, rt.Config().MaxMemoryBytes)
	})

	t.Run("default config has standard execution timeout", func(t *testing.T) {
		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		assert.Equal(t, runtime.DefaultMaxExecTime, rt.Config().MaxExecutionTime)
	})
}

func TestNewRuntimeLogging(t *testing.T) {
	t.Run("logs initialization message with version", func(t *testing.T) {
		var logMessages []string
		logger := func(msg string) {
			logMessages = append(logMessages, msg)
		}

		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{Logger: logger})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })

		require.Len(t, logMessages, 1)
		assert.True(t, strings.Contains(logMessages[0], "wazero runtime initialized successfully"))
		assert.True(t, strings.Contains(logMessages[0], rt.GetVersion()))
	})
}

func TestNewRuntimeNilLogger(t *testing.T) {
	t.Run("does not panic with nil logger", func(t *testing.T) {
		ctx := context.Background()
		rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{Logger: nil})
		require.NoError(t, err)
		t.Cleanup(func() { _ = rt.Close(ctx) })
	})
}
