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
	outputDir string
	apiKey    string
	baseURL   string
	output    string
}

func newExportCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts exportOptions

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export all content to a tar.gz archive",
		Long: `Export all content as Hugo-compatible source files.

Downloads a tar.gz archive from the server containing:
  - content/            Hugo-compatible source files (YAML frontmatter + HTML body)
  - static/uploads/media/  Bundled media files

  lesstruct-cli export
  lesstruct-cli export --output-dir /path/to/site`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runExport(cmd, opts)
		},
	}

	exportCmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the export archive")

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

	f, err := os.Create(filePath)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to create file: %s", err)}
	}
	defer func() { _ = f.Close() }()

	if err := c.ExportContent(cmd.Context(), f); err != nil {
		_ = os.Remove(filePath)
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	if err := f.Close(); err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to close file: %s", err)}
	}

	if outFormat == "json" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `{"message":"export complete","path":%q}`, filePath)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Export complete: %s\n", filePath)
	}

	return nil
}
