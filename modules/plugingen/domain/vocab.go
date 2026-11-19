package plugingen

import (
	"slices"
	"strings"
)

const (
	// FindingSeverityError blocks install.
	FindingSeverityError = "error"
	// FindingSeverityWarning surfaces in the UI without blocking.
	FindingSeverityWarning = "warning"
)

var (
	knownHooks = []string{
		"before_save",
		"after_publish",
		"before_delete",
		"after_create",
		"after_unpublish",
		"on_plugin_loaded",
	}
	knownCapabilities = []string{
		"http_get",
		"http_post",
		"db_query",
		"db_exec",
		"log_info",
		"log_error",
	}
	knownDBPermissions = []string{
		"read:content",
		"read:media",
		"read:users",
		"write:content",
		"write:media",
		"write:users",
	}
)

// IsKnownHook reports whether hook is a known WASM export suffix.
func IsKnownHook(hook string) bool {
	trimmed := strings.TrimSpace(hook)
	return slices.Contains(knownHooks, trimmed)
}

// IsKnownCapability reports whether capability names a known host function.
func IsKnownCapability(capability string) bool {
	trimmed := strings.TrimSpace(capability)
	return slices.Contains(knownCapabilities, trimmed)
}

// IsKnownDBPermission reports whether permission is a grantable database
// permission from the capability manifest vocabulary.
func IsKnownDBPermission(permission string) bool {
	trimmed := strings.TrimSpace(permission)
	return slices.Contains(knownDBPermissions, trimmed)
}

// IsValidHTTPPattern reports whether pattern is a well-formed capability
// manifest grant: an https URL on a host without userinfo, optionally ending
// in a path-scoped trailing wildcard. The rule mirrors the host's
// matchPattern: only a trailing "/..." carries wildcard meaning, so a grant
// like "https://a*" — which would authorize every host starting with
// "a" — is rejected, as is cleartext http.
func IsValidHTTPPattern(pattern string) bool {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return false
	}
	if strings.ContainsAny(trimmed, "\"\\\n\r\t ") {
		return false
	}
	if !strings.HasPrefix(trimmed, "https://") {
		return false
	}
	rest := strings.TrimPrefix(trimmed, "https://")
	if rest == "" || strings.HasPrefix(rest, "/") {
		return false
	}
	host := rest
	if h, _, ok := strings.Cut(rest, "/"); ok {
		host = h
	}
	if host == "" || strings.Contains(host, "@") {
		return false
	}
	if strings.Count(trimmed, "*") > 1 {
		return false
	}
	if strings.Contains(trimmed, "*") && !strings.HasSuffix(trimmed, "/*") {
		return false
	}
	return len(trimmed) > len("https://")+1
}

// KnownHooks returns the canonical hook suffix list.
func KnownHooks() []string {
	return slices.Clone(knownHooks)
}

// KnownCapabilities returns the canonical host-function list.
func KnownCapabilities() []string {
	return slices.Clone(knownCapabilities)
}

// KnownDBPermissions returns the canonical database permission list.
func KnownDBPermissions() []string {
	return slices.Clone(knownDBPermissions)
}
