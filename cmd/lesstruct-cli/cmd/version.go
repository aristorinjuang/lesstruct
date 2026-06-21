package cmd

// version is the CLI build version. It defaults to "dev" for local `go build`
// and `go test`; release builds inject a git-derived value via -ldflags (see the
// Makefile build-cli/install targets), so `lesstruct-cli --version` reports the
// shipped version.
var version = "dev"
