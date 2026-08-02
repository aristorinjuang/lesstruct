package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/internal/client"
	"github.com/spf13/cobra"
)

type ssgOptions struct {
	outputDir string
	apiKey    string
	baseURL   string
	output    string
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

Usage:
  lesstruct-cli ssg
  lesstruct-cli ssg --output-dir /path/to/site`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runSSG(cmd, opts)
		},
	}

	ssgCmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the site archive")

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
	filename := fmt.Sprintf("lesstruct-site-%s.tar.gz", timestamp)
	filePath := filepath.Join(opts.outputDir, filename)

	f, err := os.Create(filePath)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to create file: %s", err)}
	}
	defer func() { _ = f.Close() }()

	if err := c.GenerateSite(cmd.Context(), f); err != nil {
		_ = os.Remove(filePath)
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	if err := f.Close(); err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to close file: %s", err)}
	}

	if outFormat == "json" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `{"message":"site generation complete","path":%q}`, filePath)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Site generation complete: %s\n", filePath)
	}

	return nil
}
