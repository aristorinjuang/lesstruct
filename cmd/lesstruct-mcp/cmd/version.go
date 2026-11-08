package cmd

// version is the MCP build version. It defaults to "dev" for local `go build`
// and `go test`; release builds inject a git-derived value via -ldflags (see the
// Makefile build-mcp target), so `lesstruct-mcp --version` reports the shipped
// version.
var version = "dev"
