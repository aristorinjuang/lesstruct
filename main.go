// Command lesstruct is the default (blog) edition: every first-party
// module, every database driver. Server-only deployments build this same
// package with `-tags nosqlite` (make build-server) — postgres/mysql, no
// sqlite. The headless engine edition lives at ./cmd/lesstruct-core.
package main

import (
	"fmt"
	"os"

	"github.com/aristorinjuang/lesstruct/internal/app"
	bloggedition "github.com/aristorinjuang/lesstruct/internal/editions/blog"
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
	app.Run(bloggedition.Edition())
}
