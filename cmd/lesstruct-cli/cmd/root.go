// Package cmd holds the cobra command tree for the lesstruct-cli binary. It
// resolves credentials/config, wires the typed HTTP client, and maps outcomes to
// the documented exit-code scheme. main.go only calls Execute and os.Exit.
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
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
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(
		&apiKey,
		"api-key",
		"",
		"API key (lesstruct_<keyID>_<secret>); overrides LESSTRUCT_API_KEY and the config file. Visible in the process list — prefer the env var, the config file, or --api-key-stdin for interactive use",
	)
	root.PersistentFlags().Bool(
		"api-key-stdin",
		false,
		"read the API key from stdin's first line instead of the command line (for scripts; cannot be combined with --api-key)",
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
		"output format: text or json (json also renders errors as a JSON envelope)",
	)

	root.AddCommand(newContentCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newMediaCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newCatalogCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newOrdersCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newPaymentsCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newUsersCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newInvoiceSettingsCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newCommentCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newImportCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newExportCmd(&apiKey, &baseURL, &output))
	root.AddCommand(newSSGCmd(&apiKey, &baseURL, &output))
	return root
}

// wantsStdinKey reports whether --api-key-stdin requests the API key from
// stdin. Scanning stops at "--" so a positional value that happens to look
// like the flag is never mistaken for it.
func wantsStdinKey(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--api-key-stdin" {
			return true
		}
		if v, ok := strings.CutPrefix(a, "--api-key-stdin="); ok {
			lowered := strings.ToLower(v)
			return lowered != "" && lowered != "false" && lowered != "0" && lowered != "f"
		}
	}
	return false
}

// hasExplicitAPIKey reports whether --api-key carries a value, which
// conflicts with --api-key-stdin. A bare trailing --api-key counts as
// present: cobra itself rejects the missing value as a usage error.
func hasExplicitAPIKey(args []string) bool {
	for i, a := range args {
		if a == "--" {
			return false
		}
		if a == "--api-key" {
			return i+1 >= len(args) || !strings.HasPrefix(args[i+1], "-")
		}
		if v, ok := strings.CutPrefix(a, "--api-key="); ok {
			return v != ""
		}
	}
	return false
}

// readAPIKeyStdin consumes the API key from the first stdin line and returns
// a reader over the remainder, so commands that also read bodies from stdin
// (e.g. `content create`) see everything after the key line. An empty first
// line is a usage error, not a silent empty key.
func readAPIKeyStdin(in io.Reader) (string, io.Reader, error) {
	buf := bufio.NewReader(in)
	line, err := buf.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", buf, fmt.Errorf("--api-key-stdin found no key on stdin")
	}
	if key := strings.TrimSpace(line); key != "" {
		return key, buf, nil
	}
	return "", buf, fmt.Errorf("--api-key-stdin found no key on stdin")
}

// warnInsecureConfig warns once per invocation when the configured key file
// is group/world-readable: any local user could read the key it holds.
// This is a property of the file itself, not of this run's key source —
// even when --api-key or LESSTRUCT_API_KEY takes precedence, a readable
// key-bearing config.toml still leaks credentials to other local users.
// The warning fires only when the file actually carries a key; a keyless
// file (base URL only) is not worth nagging about. Skipped on Windows,
// where Go synthesizes permissive-looking mode bits that would false-positive
// on every file.
func warnInsecureConfig(errOut io.Writer) {
	path, ok := configFilePath()
	if !ok {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm()&0o077 == 0 {
		return
	}
	cfg, err := loadConfigFile()
	if err != nil || strings.TrimSpace(cfg.Default.APIKey) == "" {
		return
	}
	_, _ = fmt.Fprintf(errOut, "lesstruct-cli: warning: config file %s is readable by other users (mode %04o); run `chmod 600 %s` to keep your API key private\n", path, info.Mode().Perm(), path)
}

// requestedOutput sniffs the --output flag from the raw args so failures
// print in the requested mode. Cobra only binds the value on successful
// parse, but failures are exactly when the mode matters; a missing or
// malformed flag falls back to text.
func requestedOutput(args []string) string {
	for i, arg := range args {
		if arg == "--output" && i+1 < len(args) {
			return args[i+1]
		}
		if value, ok := strings.CutPrefix(arg, "--output="); ok {
			return value
		}
	}
	return "text"
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
// With --output json, failures print a JSON error envelope instead of plain
// text, so pipelines parsing stdout-as-JSON do not break on the first
// failure; the envelope carries the message and the process exit code.
// With --api-key-stdin, the key is read from stdin's first line before any
// command runs, so scripts need not put secrets on the command line.
func ExecuteArgs(args []string, in io.Reader, out, errOut io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)

	if wantsStdinKey(args) {
		if hasExplicitAPIKey(args) {
			_, _ = fmt.Fprintln(errOut, "lesstruct-cli: --api-key-stdin cannot be combined with --api-key")
			return client.ExitUsage
		}
		key, rest, err := readAPIKeyStdin(in)
		if err != nil {
			_, _ = fmt.Fprintln(errOut, "lesstruct-cli: "+err.Error())
			return client.ExitUsage
		}
		in = rest
		if serr := root.PersistentFlags().Set("api-key", key); serr != nil {
			_, _ = fmt.Fprintln(errOut, "lesstruct-cli: "+serr.Error())
			return client.ExitUsage
		}
	}

	warnInsecureConfig(errOut)

	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)

	err := root.Execute()
	if err == nil {
		return client.ExitOK
	}
	jsonMode := requestedOutput(args) == "json"
	if ee, ok := errors.AsType[*exitError](err); ok {
		if ee.msg != "" {
			if jsonMode {
				_, _ = fmt.Fprintf(errOut, "{\"error\":{\"message\":%q,\"exitCode\":%d}}\n", ee.msg, ee.code)
			} else {
				_, _ = fmt.Fprintln(errOut, ee.msg)
			}
		}
		return ee.code
	}
	if jsonMode {
		_, _ = fmt.Fprintf(errOut, "{\"error\":{\"message\":%q,\"exitCode\":%d}}\n", err.Error(), client.ExitUsage)
		return client.ExitUsage
	}
	_, _ = fmt.Fprintln(errOut, err)
	return client.ExitUsage
}
