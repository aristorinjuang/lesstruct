package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

type importOptions struct {
	source       string
	skipMedia    bool
	pollInterval string
	apiKey       string
	baseURL      string
	output       string
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

The source may be a Hugo project directory or a .tar.gz archive of one.
Directories are archived automatically (content/ and static/ only) before
upload. The import runs asynchronously on the server; the CLI polls the job
status and prints live progress:
  lesstruct-cli import hugo --source /path/to/hugo-site
  lesstruct-cli import hugo --source hugo-site.tar.gz
`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runImportHugo(cmd, opts)
		},
	}

	hugoCmd.Flags().StringVar(&opts.source, "source", "", "Path to Hugo project directory or .tar.gz archive")
	_ = hugoCmd.MarkFlagRequired("source")
	hugoCmd.Flags().BoolVar(
		&opts.skipMedia,
		"skip-media",
		false,
		"skip migrating static/ images into Lesstruct media; images stay linked to original paths/URLs",
	)
	hugoCmd.Flags().StringVar(
		&opts.pollInterval,
		"poll-interval",
		"3s",
		"polling interval for import progress (e.g. 1s, 5s)",
	)

	return hugoCmd
}

// buildHugoArchive opens --source (a .tar.gz archive) or archives a Hugo
// project directory (content/ and static/ only) into a temp tar.gz file. The
// caller is responsible for removing the returned file.
func buildHugoArchive(source string) (string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", fmt.Errorf("read --source %q: %w", source, err)
	}

	if !info.IsDir() {
		if !strings.HasSuffix(strings.ToLower(source), ".tar.gz") && !strings.HasSuffix(strings.ToLower(source), ".tgz") {
			return "", fmt.Errorf("--source %q is not a directory and not a .tar.gz archive", source)
		}
		return source, nil
	}

	tmpFile, err := os.CreateTemp("", "hugo-import-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("create temp archive: %w", err)
	}
	tmpPath := tmpFile.Name()

	if err := archiveHugoProject(source, tmpFile); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close temp archive: %w", err)
	}
	return tmpPath, nil
}

// archiveHugoProject writes content/ and static/ (when present) of a Hugo
// project directory into w as a tar.gz archive.
func archiveHugoProject(projectDir string, w io.Writer) error {
	gzWriter := gzip.NewWriter(w)
	tarWriter := tar.NewWriter(gzWriter)

	writeDir := func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				// static/ is optional — a Hugo project may not have one.
				return nil
			}
			return fmt.Errorf("read %s: %w", dir, err)
		}
		if len(entries) == 0 {
			return nil
		}
		return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = rel
			if d.IsDir() {
				header.Name += "/"
			}
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}
			if !d.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(tarWriter, f)
				_ = f.Close()
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	if err := writeDir(filepath.Join(projectDir, "content")); err != nil {
		return err
	}
	if err := writeDir(filepath.Join(projectDir, "static")); err != nil {
		return err
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("finalize archive: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("finalize archive: %w", err)
	}
	return nil
}

func runImportHugo(cmd *cobra.Command, opts importOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	if opts.source == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --source is required",
		}
	}

	parsedDuration, err := time.ParseDuration(opts.pollInterval)
	if err != nil || parsedDuration <= 0 {
		return &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --poll-interval %q (e.g. 1s, 5s)", opts.pollInterval),
		}
	}

	archivePath, err := buildHugoArchive(opts.source)
	if err != nil {
		return &exitError{code: client.ExitValidation, msg: fmt.Sprintf("lesstruct-cli: %s", err)}
	}
	defer func() {
		if archivePath != opts.source {
			_ = os.Remove(archivePath)
		}
	}()

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return &exitError{
			code: client.ExitValidation,
			msg:  fmt.Sprintf("lesstruct-cli: open archive %q: %s", archivePath, err),
		}
	}
	defer func() { _ = archiveFile.Close() }()

	apiKey, baseURL, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	cl, err := client.New(baseURL, apiKey)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	// Upload the archive.
	data, _, err := cl.ImportHugo(
		cmd.Context(),
		client.ImportHugoRequest{
			File:      archiveFile,
			Filename:  filepath.Base(archivePath),
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
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (media migration skipped)")
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
			statusRaw, _, err := cl.GetHugoImportStatus(cmd.Context(), resp.JobID)
			if err != nil {
				// Transient poll error — retry on next tick.
				continue
			}

			var env importStatusEnvelope
			if err := json.Unmarshal(statusRaw, &env); err != nil {
				continue
			}

			job := env.Job
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Progress: %d / %d items imported, %d skipped\r",
				job.Imported, job.Total, job.Skipped)

			if job.State == "done" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Import complete: %d imported, %d skipped\n",
					job.Imported, job.Skipped)
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
					msg:  "lesstruct-cli: Hugo import job failed",
				}
			}
		}
	}
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
