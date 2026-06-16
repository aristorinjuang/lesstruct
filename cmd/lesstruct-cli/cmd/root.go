// Package cmd holds the cobra command tree for the lesstruct-cli binary. It
// resolves credentials/config, wires the typed HTTP client, and maps outcomes to
// the documented exit-code scheme. main.go only calls Execute and os.Exit.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

// exitError carries an exit code plus a stderr diagnostic. Subcommand RunE
// functions return *exitError; ExecuteArgs extracts the code and prints the
// message to stderr.
type exitError struct {
	code int
	msg  string
}

// Error implements the error interface.
func (e *exitError) Error() string {
	return e.msg
}

// newRootCmd builds a fresh root command tree binding the persistent flags to
// local variables. Building fresh per ExecuteArgs call avoids cross-run flag
// leakage (cobra does not reset bound vars to defaults on re-parse) and keeps
// the command tree easy to drive from tests.
func newRootCmd() *cobra.Command {
	var apiKey, baseURL, output string

	root := &cobra.Command{
		Use:           "lesstruct-cli",
		Short:         "Author and manage Lesstruct content and media over the /api/v1 API",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(
		&apiKey,
		"api-key",
		"",
		"API key (lesstruct_<keyID>_<secret>); overrides LESSTRUCT_API_KEY and the config file",
	)
	root.PersistentFlags().StringVar(
		&baseURL,
		"base-url",
		"",
		"API base URL; overrides LESSTRUCT_BASE_URL and the config file",
	)
	root.PersistentFlags().StringVar(
		&output,
		"output",
		"text",
		"output format: text or json",
	)

	root.AddCommand(newContentCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newMediaCmd(&apiKey, &baseURL, &output))
	return root
}

// Execute runs the CLI against os.Args and os.Stdin/Stdout/Stderr, returning
// the process exit code. main.go calls this and os.Exits with it.
func Execute() int {
	return ExecuteArgs(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

// ExecuteArgs runs the CLI with explicit args and IO streams, returning the
// exit code. Exported so tests can drive a freshly-built command tree
// deterministically. A *exitError yields its code (and its message to errOut);
// any other cobra error (bad flags, unknown command) is treated as usage (2).
func ExecuteArgs(args []string, in io.Reader, out, errOut io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)

	err := root.Execute()
	if err == nil {
		return client.ExitOK
	}
	if ee, ok := errors.AsType[*exitError](err); ok {
		if ee.msg != "" {
			_, _ = fmt.Fprintln(errOut, ee.msg)
		}
		return ee.code
	}
	_, _ = fmt.Fprintln(errOut, err)
	return client.ExitUsage
}
