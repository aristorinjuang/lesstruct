// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for comments.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions: the comments
// moderation navigation entry. Create/list/moderation surfaces render from
// the comments API; no settings sections or editor extensions are declared.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{
		MenuEntries: []kernel.AdminMenuEntry{
			{Label: "Comments", Path: "/comment", Icon: "chat-bubble"},
		},
	}
}
