package plugin

import (
	"context"
	"errors"
)

type HookName string

func (h HookName) String() string {
	return string(h)
}

const (
	HookBeforeSave     HookName = "BeforeSaveContent"
	HookAfterPublish   HookName = "AfterPublishContent"
	HookBeforeDelete   HookName = "BeforeDeleteContent"
	HookAfterCreate    HookName = "AfterCreateContent"
	HookOnPluginLoaded HookName = "OnPluginLoaded"
)

const DefaultPriority = 100

type FailureMode int

const (
	FailFast       FailureMode = iota
	LogAndContinue
	Fallback
)

const DefaultFailureMode FailureMode = FailFast

func (f FailureMode) String() string {
	switch f {
	case LogAndContinue:
		return "log-and-continue"
	case Fallback:
		return "fallback"
	default:
		return "fail-fast"
	}
}

func ParseFailureMode(s string) FailureMode {
	switch s {
	case "log-and-continue":
		return LogAndContinue
	case "fallback":
		return Fallback
	default:
		return FailFast
	}
}

var wasmHookMapping = map[string]HookName{
	"before_save":      HookBeforeSave,
	"after_publish":    HookAfterPublish,
	"before_delete":    HookBeforeDelete,
	"after_create":     HookAfterCreate,
	"on_plugin_loaded": HookOnPluginLoaded,
}

type HookHandler func(ctx context.Context, data []byte) ([]byte, error)

type HookRegistration struct {
	PluginName     string
	HookName       HookName
	Priority       int
	Handler        HookHandler
	FailureMode    FailureMode
	FallbackHandler HookHandler
}

var (
	ErrHookNotFound          = errors.New("hook not found")
	ErrHookExecutionFailed   = errors.New("hook execution failed")
	ErrHookAlreadyRegistered = errors.New("hook already registered")
)

func ResolveWasmHookName(exportSuffix string) HookName {
	if canonical, ok := wasmHookMapping[exportSuffix]; ok {
		return canonical
	}
	return HookName(exportSuffix)
}
