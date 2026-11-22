// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for module generation.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions: the module
// generator and staged-projects navigation entries.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{
		MenuEntries: []kernel.AdminMenuEntry{
			{Label: "Module Generator", Path: "/module-generator", Icon: "package"},
			{Label: "Modules", Path: "/modules", Icon: "blocks"},
		},
	}
}
