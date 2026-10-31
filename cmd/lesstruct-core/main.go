// Command lesstruct-core is the Core edition: the headless engine. Identity
// (users/auth/adminpanel), the content engine, post types, media, and the
// plugin system — no comment system, no SEO surfaces, no SSR theme engine,
// no AI text generation, no importers. The blog edition is the root
// package; this binary links only what Core carries.
package main

import (
	"github.com/aristorinjuang/lesstruct/internal/app"
	coreedition "github.com/aristorinjuang/lesstruct/internal/editions/core"
)

func main() {
	app.Run(coreedition.Edition())
}
