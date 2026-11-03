// Package admin owns this module's admin-shell extensions: the declarative UI
// the single admin SPA renders for voice dictation.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions. The Dictate button
// renders data-driven from the /api/health feature flags, so the module
// declares no menu entries, settings sections, or editor extensions.
// The empty manifest still validates, which is the contract the shell checks.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}
