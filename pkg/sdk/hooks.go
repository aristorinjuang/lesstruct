package sdk

// Hook name constants matching the canonical WASM export suffixes.
// Plugin .wasm files MUST export functions named "hook_" + these values.
const (
	HookBeforeSave     = "before_save"
	HookAfterPublish   = "after_publish"
	HookBeforeDelete   = "before_delete"
	HookAfterCreate    = "after_create"
	HookOnPluginLoaded = "on_plugin_loaded"
)

// DefaultPriority is the default execution priority for hooks.
// Lower values run earlier. Default is 100.
const DefaultPriority = 100

// FailureMode determines how the plugin system handles hook failures.
type FailureMode int

const (
	FailFast       FailureMode = iota // Stop execution on failure
	LogAndContinue                    // Log error and continue to next hook
	Fallback                          // Use fallback handler on failure
)
