// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for content imports.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions: the Import section,
// whose WordPress and Hugo pages are served by the static admin router.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{
		MenuEntries: []kernel.AdminMenuEntry{
			{Label: "Import", Path: "/import", Icon: "arrow-down-tray"},
		},
	}
}
