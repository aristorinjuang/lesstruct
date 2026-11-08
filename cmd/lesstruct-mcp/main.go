// Command lesstruct-mcp exposes Lesstruct to AI agents over the Model
// Context Protocol (MCP). It speaks stdio — the agent spawns this binary and
// talks JSON-RPC over stdin/stdout — and forwards every tool call to the
// versioned /api/v1 REST API with the operator's API key. Run
// `lesstruct-mcp --help` for setup.
package main

import (
	"os"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-mcp/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
