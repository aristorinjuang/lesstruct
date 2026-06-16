package sdk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sdkpkg "github.com/aristorinjuang/lesstruct/pkg/sdk"
)

func TestHookConstants(t *testing.T) {
	assert.Equal(t, "before_save", sdkpkg.HookBeforeSave)
	assert.Equal(t, "after_publish", sdkpkg.HookAfterPublish)
	assert.Equal(t, "before_delete", sdkpkg.HookBeforeDelete)
	assert.Equal(t, "after_create", sdkpkg.HookAfterCreate)
	assert.Equal(t, "on_plugin_loaded", sdkpkg.HookOnPluginLoaded)
}

func TestDefaultPriority(t *testing.T) {
	assert.Equal(t, 100, sdkpkg.DefaultPriority)
}

func TestFailureModeValues(t *testing.T) {
	assert.Equal(t, sdkpkg.FailureMode(0), sdkpkg.FailFast)
	assert.Equal(t, sdkpkg.FailureMode(1), sdkpkg.LogAndContinue)
	assert.Equal(t, sdkpkg.FailureMode(2), sdkpkg.Fallback)
}
