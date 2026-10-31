// Command lesstruct is the default (blog) edition: every first-party
// module, every database driver. Server-only deployments build this same
// package with `-tags nosqlite` (make build-server) — postgres/mysql, no
// sqlite. The headless engine edition lives at ./cmd/lesstruct-core.
package main

import (
	"github.com/aristorinjuang/lesstruct/internal/app"
	bloggedition "github.com/aristorinjuang/lesstruct/internal/editions/blog"
)

func main() {
	app.Run(bloggedition.Edition())
}
