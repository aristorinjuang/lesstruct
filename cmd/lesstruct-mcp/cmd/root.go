package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// newRootCmd builds the lesstruct-mcp command. It takes no subcommands: with
// no flags it resolves credentials and serves MCP over stdio until the client
// disconnects. Stdout is protocol-only — every diagnostic goes to errOut.
func newRootCmd() *cobra.Command {
	var apiKey, baseURL string

	root := &cobra.Command{
		Use:           "lesstruct-mcp",
		Short:         "Expose Lesstruct to AI agents over MCP (stdio)",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cmd.ErrOrStderr(), apiKey, baseURL)
		},
	}
	root.Flags().StringVar(
		&apiKey,
		"api-key",
		"",
		"API key (lesstruct_<keyID>_<secret>); overrides LESSTRUCT_API_KEY and the config file. Never logged — only the key ID appears in diagnostics",
	)
	root.Flags().StringVar(
		&baseURL,
		"base-url",
		"",
		"API base URL; overrides LESSTRUCT_BASE_URL and the config file",
	)
	return root
}

// run resolves credentials, builds the tool server, and serves it over stdio
// until the client disconnects. errOut carries every diagnostic; stdout stays
// protocol-only.
func run(ctx context.Context, errOut io.Writer, apiKey, baseURL string) error {
	key, url, err := resolveCredentials(apiKey, baseURL)
	if err != nil {
		return err
	}

	warnInsecureConfig(errOut)

	srv, err := New(key, url)
	if err != nil {
		return fmt.Errorf("lesstruct-mcp: %w", err)
	}
	return srv.Run(ctx)
}

// warnInsecureConfig warns once per invocation when the configured key file is
// group/world-readable: any local user could read the key it holds. The warning
// fires only when the file actually carries a key; a keyless file (base URL
// only) is not worth nagging about. Skipped on Windows, where Go synthesizes
// permissive-looking mode bits that would false-positive on every file.
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
	_, _ = fmt.Fprintf(errOut, "lesstruct-mcp: warning: config file %s is readable by other users (mode %04o); run `chmod 600 %s` to keep your API key private\n", path, info.Mode().Perm(), path)
}

// Execute runs the MCP server against os.Args with stdout as the protocol
// stream and stderr for diagnostics, returning the process exit code.
// main.go calls this and os.Exits with it.
func Execute() int {
	return ExecuteArgs(os.Args[1:], os.Stdout, os.Stderr)
}

// ExecuteArgs runs the MCP server with explicit args and IO streams, returning
// the exit code. Exported so tests can drive flag parsing and the credential
// chain deterministically. A successful --version or a flag-parsing failure
// returns without touching the network; a valid credential set blocks serving
// stdio until the client disconnects.
func ExecuteArgs(args []string, out, errOut io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errOut)

	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(errOut, err.Error())
		return 1
	}
	return 0
}
