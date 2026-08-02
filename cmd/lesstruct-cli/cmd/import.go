package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

type importOptions struct {
	source  string
	apiKey  string
	baseURL string
	output  string
}

// importJobResponse is the envelope data from POST /api/v1/wordpress/import.
type importJobResponse struct {
	JobID string `json:"jobId"`
	State string `json:"state"`
}

// importStatusEnvelope is the nested shape from GET .../import/status/{jobId}.
type importStatusEnvelope struct {
	Job client.ImportJobStatus `json:"job"`
}

func newImportCmd(apiKey, baseURL, output *string) *cobra.Command {
	importCmd := &cobra.Command{
		Use:           "import",
		Short:         "Import content from external sources",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	importCmd.AddCommand(newImportHugoCmd(apiKey, baseURL, output))
	importCmd.AddCommand(newImportWordpressCmd(apiKey, baseURL, output))
	return importCmd
}

func newImportHugoCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts importOptions

	hugoCmd := &cobra.Command{
		Use:   "hugo --source <path>",
		Short: "Import a Hugo site",
		Long: `Import all content from a Hugo site.

Upload a tar.gz archive of the Hugo project to the admin panel:
  lesstruct-cli import hugo --source /path/to/hugo-site

Or use curl directly:
  curl -X POST <baseURL>/api/admin/hugo/import -F "file=@hugo-site.tar.gz" -b "token=..."

The archive should contain at minimum a content/ directory with your
Hugo posts (HTML or Markdown files with YAML frontmatter).
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runImportHugo(cmd, opts)
		},
	}

	hugoCmd.Flags().StringVar(&opts.source, "source", "", "Path to Hugo project directory or .tar.gz archive")
	_ = hugoCmd.MarkFlagRequired("source")

	return hugoCmd
}

func runImportHugo(cmd *cobra.Command, opts importOptions) error {
	output := opts.output
	if output != "text" && output != "json" {
		return &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --output %q (want \"text\" or \"json\")", output),
		}
	}

	if opts.source == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --source is required",
		}
	}

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	if output == "json" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "{\"message\":\"To import, upload the Hugo archive to the admin panel\",\"source\":%q,\"endpoint\":\"%s/api/admin/hugo/import\"}\n", opts.source, baseURL)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Hugo import from %s\n\n", opts.source)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "To import, upload the archive to the admin panel:\n")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  curl -X POST %s/api/admin/hugo/import -F \"file=@%s\" -b \"token=...\"\n\n", baseURL, apiKey)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Or tar.gz the project and upload via the admin UI.\n")
	}

	return nil
}

func newImportWordpressCmd(apiKey, baseURL, output *string) *cobra.Command {
	var file, pollInterval string
	var skipMedia bool

	wc := &cobra.Command{
		Use:           "wordpress --file <path>",
		Short:         "Import a WordPress WXR export file",
		Long: "Import a WordPress eXtended RSS (WXR) export file via " +
			"/api/v1/wordpress/import. Uploads the file, then polls the " +
			"import status until the job completes, printing live progress. " +
			"Use --skip-media to import text only (images stay linked to " +
			"the original WordPress URLs).",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportWordpress(cmd, importWordpressOptions{
				apiKey:       *apiKey,
				baseURL:      *baseURL,
				output:       *output,
				file:         file,
				pollInterval: pollInterval,
				skipMedia:    skipMedia,
			})
		},
	}

	wc.Flags().StringVar(
		&file,
		"file",
		"",
		"path to the WordPress WXR export file (.xml) (required)",
	)
	wc.Flags().BoolVar(
		&skipMedia,
		"skip-media",
		false,
		"skip downloading media during import; images stay linked to original WordPress URLs",
	)
	wc.Flags().StringVar(
		&pollInterval,
		"poll-interval",
		"3s",
		"polling interval for import progress (e.g. 1s, 5s)",
	)

	return wc
}

type importWordpressOptions struct {
	apiKey       string
	baseURL      string
	output       string
	file         string
	pollInterval string
	skipMedia    bool
}

func runImportWordpress(cmd *cobra.Command, opts importWordpressOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	if opts.file == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --file is required",
		}
	}

	parsedDuration, err := time.ParseDuration(opts.pollInterval)
	if err != nil || parsedDuration <= 0 {
		return &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --poll-interval %q (e.g. 1s, 5s)", opts.pollInterval),
		}
	}

	fileInfo, err := os.Stat(opts.file)
	if err != nil {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: read --file %q: %s", opts.file, err),
		}
	}
	if fileInfo.IsDir() {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: --file %q is a directory", opts.file),
		}
	}
	if !strings.HasSuffix(strings.ToLower(opts.file), ".xml") {
		return &exitError{
			code: client.ExitValidation,
			msg:  "lesstruct-cli: --file must be a WordPress export (.xml)",
		}
	}

	f, err := os.Open(opts.file)
	if err != nil {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: read --file %q: %s", opts.file, err),
		}
	}
	defer func() { _ = f.Close() }()

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	// Upload the file.
	data, _, err := cl.ImportWordPress(
		cmd.Context(),
		client.ImportWordPressRequest{
			File:      f,
			Filename:  filepath.Base(opts.file),
			SkipMedia: opts.skipMedia,
		},
	)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	var resp importJobResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: unexpected response from import endpoint",
		}
	}
	if resp.JobID == "" {
		return &exitError{
			code: client.ExitGeneric,
			msg:  "lesstruct-cli: server returned no job ID",
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Import started — job ID: %s\n", resp.JobID)
	if opts.skipMedia {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (media download skipped)")
	}

	// Poll until done or failed.
	ticker := time.NewTicker(parsedDuration)
	defer ticker.Stop()

	for {
		select {
		case <-cmd.Context().Done():
			return &exitError{
				code: client.ExitGeneric,
				msg:  "lesstruct-cli: import cancelled",
			}
		case <-ticker.C:
			statusRaw, _, err := cl.GetImportStatus(cmd.Context(), resp.JobID)
			if err != nil {
				// Transient poll error — retry on next tick.
				continue
			}

			var env importStatusEnvelope
			if err := json.Unmarshal(statusRaw, &env); err != nil {
				continue
			}

			job := env.Job
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Progress: %d / %d items imported, %d skipped, %d users created\r",
				job.Imported, job.Total, job.Skipped, job.UsersImported)

			if job.State == "done" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Import complete: %d imported, %d skipped, %d users created\n",
					job.Imported, job.Skipped, job.UsersImported)
				if len(job.Errors) > 0 {
					limit := len(job.Errors)
					if limit > 10 {
						limit = 10
					}
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %d issue(s) encountered:\n", len(job.Errors))
					for _, e := range job.Errors[:limit] {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "    - %s\n", e)
					}
					if len(job.Errors) > 10 {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "    ... and %d more\n", len(job.Errors)-10)
					}
				}
				return nil
			}

			if job.State == "failed" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Import failed after importing %d items, %d skipped\n",
					job.Imported, job.Skipped)
				if len(job.Errors) > 0 {
					limit := len(job.Errors)
					if limit > 10 {
						limit = 10
					}
					for _, e := range job.Errors[:limit] {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", e)
					}
					if len(job.Errors) > 10 {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  ... and %d more\n", len(job.Errors)-10)
					}
				}
				return &exitError{
					code: client.ExitGeneric,
					msg:  "lesstruct-cli: WordPress import job failed",
				}
			}
		}
	}
}
