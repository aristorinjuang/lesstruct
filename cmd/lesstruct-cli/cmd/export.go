package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

type exportOptions struct {
	outputDir       string
	timeout         string
	maxDownloadSize string
	apiKey          string
	baseURL         string
	output          string
}

func newExportCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts exportOptions

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export all content to a tar.gz archive",
		Long: `Export all content as Hugo-compatible source files.

Downloads a tar.gz archive from the server containing:
  - content/            Hugo-compatible source files (YAML frontmatter + HTML body)
  - static/uploads/media/  Media files referenced by the exported content
                          (deduplicated; unreferenced library files are not
                          included)

  lesstruct-cli export
  lesstruct-cli export --output-dir /path/to/site
  lesstruct-cli export --timeout 10m --max-download-size 500MB`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runExport(cmd, opts)
		},
	}

	exportCmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the export archive")
	exportCmd.Flags().StringVar(&opts.timeout, "timeout", "30m", "Overall download deadline (Go duration, e.g. 30m, 2h; 0 disables)")
	exportCmd.Flags().StringVar(&opts.maxDownloadSize, "max-download-size", "2GiB", "Download size cap (bytes, or K/M/G suffixes like 500MB; 0 disables)")

	return exportCmd
}

func runExport(cmd *cobra.Command, opts exportOptions) error {
	outFormat := opts.output
	if outFormat != "text" && outFormat != "json" {
		return &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --output %q (want \"text\" or \"json\")", outFormat),
		}
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	c, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	now := time.Now()
	timestamp := now.Format("2006-01-02-150405")
	filename := fmt.Sprintf("lesstruct-export-%s.tar.gz", timestamp)
	filePath := filepath.Join(opts.outputDir, filename)

	// Download into a .part file and rename on success, so an interrupted
	// or capped download never leaves a truncated archive masquerading as a
	// complete export.
	partPath := filePath + ".part"
	f, err := os.Create(partPath)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to create file: %s", err)}
	}
	defer func() { _ = f.Close() }()

	// NOTE: derr is deliberately not err: parseDownloadOptions returns a
	// *exitError, and assigning it into the error-typed err in scope would
	// box a nil *exitError into a non-nil error interface.
	downloadOpts, derr := parseDownloadOptions(opts.timeout, opts.maxDownloadSize)
	if derr != nil {
		_ = f.Close()
		_ = os.Remove(partPath)
		return derr
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	if err := c.ExportContent(ctx, f, downloadOpts); err != nil {
		_ = f.Close()
		_ = os.Remove(partPath)
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(partPath)
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to close file: %s", err)}
	}
	if err := os.Rename(partPath, filePath); err != nil {
		_ = os.Remove(partPath)
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to finalize archive: %s", err)}
	}

	if outFormat == "json" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `{"message":"export complete","path":%q}`, filePath)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Export complete: %s\n", filePath)
	}

	return nil
}
