// Package core is the Lesstruct Core edition: the headless engine. Identity
// (users/auth/adminpanel), the content engine, post types, media, and the
// plugin system — no comment system, no SEO surfaces, no SSR theme engine,
// no AI text generation, no importers. Sites that render their own frontend
// against /api/v1 install this edition:
//
//	go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-core@latest
package core

import "github.com/aristorinjuang/lesstruct/internal/app"

// Edition returns the Core edition. Nothing to wire beyond the platform:
// every module Core carries is platform-built, and every optional HTTP
// surface stays unmounted.
func Edition() app.Edition {
	return app.Edition{
		Name: "core",
	}
}
