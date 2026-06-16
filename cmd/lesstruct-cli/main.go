// Command lesstruct-cli authors and manages Lesstruct content and media over
// the versioned /api/v1 REST API. It is a thin client over the JSON contract —
// it imports no server internals. Run `lesstruct-cli --help` for usage.
package main

import (
	"os"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
