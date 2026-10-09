// Command lesstruct is the default (blog) edition: every first-party
// module, every database driver. Server-only deployments build this same
// package with `-tags nosqlite` (make build-server) — postgres/mysql, no
// sqlite.
package main

import (
	"github.com/aristorinjuang/lesstruct/internal/app"
)

func main() {
	app.Run()
}
