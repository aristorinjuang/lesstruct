// Command lesstruct-core is the Core edition: the headless engine. Identity
// (users/auth/adminpanel), the content engine, post types, media, and the
// plugin system — no comment system, no SEO surfaces, no SSR theme engine,
// no AI text generation, no importers. The blog edition is the root
// package; this binary links only what Core carries.
package main

import (
	"fmt"
	"os"

	"github.com/aristorinjuang/lesstruct/internal/app"
	coreedition "github.com/aristorinjuang/lesstruct/internal/editions/core"
)

// version is the build version, stamped via -ldflags -X main.version in
// release builds (see .github/workflows/release.yml); `--version` reports it
// so an operator can confirm exactly which build is running.
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println(version)
		return
	}
	app.Run(coreedition.Edition())
}
