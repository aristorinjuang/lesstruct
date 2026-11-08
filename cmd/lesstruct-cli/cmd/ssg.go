package cmd

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

type ssgOptions struct {
	outputDir       string
	extractDir      string
	timeout         string
	maxDownloadSize string
	apiKey          string
	baseURL         string
	output          string
}

func newSSGCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts ssgOptions

	ssgCmd := &cobra.Command{
		Use:   "ssg",
		Short: "Generate a static site from all published content",
		Long: `Generate a fully static HTML site with AMP variants.

Downloads a tar.gz archive from the server containing:
  - index.html, page/2/index.html, etc.  Homepage + pagination
  - <slug>/index.html                    Content pages
  - <slug>/amp/index.html                AMP variants
  - <post-type>/index.html               Post type listings
  - authors/<username>/index.html        Author pages
  - tags/<tag>/index.html                Tag pages
  - static/base.css, style.css           Theme CSS
  - uploads/media/                       Media files
  - sitemap.xml, robots.txt              SEO files
  - 404.html                             Not-found page
  - index.xml                            RSS feed of recent posts

By default the archive is written to disk as lesstruct-site-<timestamp>.tar.gz.
With --extract-dir the files are extracted straight into the given directory
instead (no archive is kept).

Usage:
  lesstruct-cli ssg
  lesstruct-cli ssg --output-dir /path/to/archive-dir
  lesstruct-cli ssg --extract-dir /path/to/site`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runSSG(cmd, opts)
		},
	}

	ssgCmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the site archive")
	ssgCmd.Flags().StringVar(&opts.extractDir, "extract-dir", "", "Directory to extract the site files into instead of keeping the archive")
	ssgCmd.Flags().StringVar(&opts.timeout, "timeout", "30m", "Overall download deadline (Go duration, e.g. 30m, 2h; 0 disables)")
	ssgCmd.Flags().StringVar(&opts.maxDownloadSize, "max-download-size", "2GiB", "Download size cap (bytes, or K/M/G suffixes like 500MB; 0 disables)")

	return ssgCmd
}

func runSSG(cmd *cobra.Command, opts ssgOptions) error {
	outFormat := opts.output
	if outFormat != "text" && outFormat != "json" {
		return &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --output %q (want \"text\" or \"json\")", outFormat),
		}
	}

	if opts.extractDir != "" && opts.outputDir != "." {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --output-dir and --extract-dir are mutually exclusive",
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

	if opts.extractDir != "" {
		return runSSGExtract(cmd, c, opts)
	}

	now := time.Now()
	timestamp := now.Format("2006-01-02-150405")
	filename := fmt.Sprintf("lesstruct-site-%s.tar.gz", timestamp)
	filePath := filepath.Join(opts.outputDir, filename)

	// Download into a .part file and rename on success, so an interrupted
	// or capped download never leaves a truncated archive masquerading as
	// a complete site.
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
	if err := c.GenerateSite(ctx, f, downloadOpts); err != nil {
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

	writeSSGResult(cmd, opts.output, "site generation complete", filePath)
	return nil
}

func runSSGExtract(cmd *cobra.Command, c *client.Client, opts ssgOptions) error {
	tmpFile, err := os.CreateTemp("", "lesstruct-ssg-*.tar.gz")
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to create temp file: %s", err)}
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	// NOTE: derr is deliberately not err: parseDownloadOptions returns a
	// *exitError, and assigning it into the error-typed err in scope would
	// box a nil *exitError into a non-nil error interface.
	downloadOpts, derr := parseDownloadOptions(opts.timeout, opts.maxDownloadSize)
	if derr != nil {
		_ = tmpFile.Close()
		return derr
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	if err := c.GenerateSite(ctx, tmpFile, downloadOpts); err != nil {
		_ = tmpFile.Close()
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}
	if err := tmpFile.Close(); err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to close temp file: %s", err)}
	}

	if err := extractSiteArchive(tmpPath, opts.extractDir); err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	writeSSGResult(cmd, opts.output, "site extracted", opts.extractDir)
	return nil
}

func writeSSGResult(cmd *cobra.Command, outFormat, message, resultPath string) {
	if outFormat == "json" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `{"message":%q,"path":%q}`+"\n", message, resultPath)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", capitalize(message), resultPath)
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func extractSiteArchive(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()
	return extractTarGz(f, destDir)
}

func extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("read gzip stream: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive entry: %w", err)
		}

		destPath, ok := safeArchivePath(destDir, header.Name)
		if !ok {
			return fmt.Errorf("archive entry %q escapes the destination directory", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", header.Name, err)
			}
		case tar.TypeReg:
			if err := writeArchiveFile(destPath, tr, os.FileMode(header.Mode)&0777); err != nil {
				return fmt.Errorf("write file %s: %w", header.Name, err)
			}
		default:
			continue
		}
	}
}

func safeArchivePath(destDir, name string) (string, bool) {
	slash := filepath.ToSlash(name)
	if slash == "" || strings.ContainsRune(slash, 0) {
		return "", false
	}
	cleaned := path.Clean(slash)
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return filepath.Join(destDir, filepath.FromSlash(cleaned)), true
}

func writeArchiveFile(destPath string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, r)
	return err
}
