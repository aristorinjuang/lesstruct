// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for media.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions: the media library
// navigation entry. Upload, list, and generation surfaces render from the
// media API; no settings sections or editor extensions are declared.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{
		MenuEntries: []kernel.AdminMenuEntry{
			{Label: "Media", Path: "/media", Icon: "photo"},
		},
	}
}
