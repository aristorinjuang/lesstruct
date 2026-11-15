package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

// defaultGenerateTimeout caps one theme generation run end to end client-side.
// The server allows 180s for template-override generations (plus the repair
// loop), so the default carries headroom; 0 disables the deadline.
const defaultGenerateTimeout = "5m"

// themeGenerateData is the cmd-layer projection of the generate response —
// it mirrors the server's {theme, validation} payload, so decoding drops
// unknown fields silently. Used by the text output; json mode re-emits the
// raw envelope via printEnvelope.
type themeGenerateData struct {
	Theme struct {
		Name      string `json:"name"`
		Edition   string `json:"edition"`
		Rationale string `json:"rationale"`
	} `json:"theme"`
	Validation struct {
		Valid    bool `json:"valid"`
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"findings"`
	} `json:"validation"`
}

// installedThemeSummary is the cmd-layer projection of an installed theme.
type installedThemeSummary struct {
	Name      string `json:"name"`
	Edition   string `json:"edition"`
	CreatedAt string `json:"createdAt"`
	Active    bool   `json:"active"`
	Source    string `json:"source"`
}

// themeActivationData is the cmd-layer projection of the activate/deactivate
// response.
type themeActivationData struct {
	Activated       bool `json:"activated"`
	RequiresRestart bool `json:"requiresRestart"`
}

// firstFindingMessage extracts the first error-severity finding from a
// generation-failure details payload. It mirrors the admin panel's toast:
// warnings are guidance, only errors explain the failure.
func firstFindingMessage(details json.RawMessage) string {
	if len(details) == 0 {
		return ""
	}
	var payload struct {
		Findings []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(details, &payload); err != nil {
		return ""
	}
	for _, finding := range payload.Findings {
		if finding.Severity == "error" && finding.Message != "" {
			return finding.Message
		}
	}
	return ""
}

// themeGenerateOptions bundles the resolved flag values for theme generate.
type themeGenerateOptions struct {
	description       string
	style             string
	templateOverrides bool
	edition           string
	timeout           string
	apiKey            string
	baseURL           string
	output            string
}

// themeNameOptions bundles the resolved flag values for the single-theme
// verbs (activate/delete) that take only a name.
type themeNameOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// themeDownloadOptions bundles the resolved flag values for theme download.
type themeDownloadOptions struct {
	outputDir       string
	timeout         string
	maxDownloadSize string
	apiKey          string
	baseURL         string
	output          string
}

// newThemeCmd builds the `theme` command tree. The persistent-flag pointers
// (apiKey/baseURL/output) are captured by each subcommand's RunE so it reads
// the parsed values at execution time.
func newThemeCmd(apiKey, baseURL, output *string) *cobra.Command {
	theme := &cobra.Command{
		Use:           "theme",
		Short:         "Generate and manage AI themes over /api/v1/themegen",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	theme.AddCommand(newThemeGenerateCmd(apiKey, baseURL, output))
	theme.AddCommand(newThemeListCmd(apiKey, baseURL, output))
	theme.AddCommand(newThemeDownloadCmd(apiKey, baseURL, output))
	theme.AddCommand(newThemeActivateCmd(apiKey, baseURL, output))
	theme.AddCommand(newThemeDeactivateCmd(apiKey, baseURL, output))
	theme.AddCommand(newThemeDeleteCmd(apiKey, baseURL, output))

	return theme
}

func newThemeGenerateCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts themeGenerateOptions

	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a theme from a natural-language brief",
		Long: `Generate a validated theme from a natural-language brief.

Stylesheet-only is the default; --template-overrides also generates page
templates, parse-tested and smoke-rendered before install. The generated
theme auto-installs as inactive; activate it with theme activate.

The run can take minutes (the repair loop retries up to twice): --timeout
(default 5m) caps the whole call, 0 disables. On a failed gate the first
error finding prints as the failure reason; warnings print as guidance.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runThemeGenerate(cmd, opts)
		},
	}

	generateCmd.Flags().StringVar(&opts.description, "description", "", "Natural-language theme brief (required, max 5,000 chars)")
	generateCmd.Flags().StringVar(&opts.style, "style", "", "Style preset: auto (default), dark, light, minimal, playful, editorial, brutalist")
	generateCmd.Flags().BoolVar(&opts.templateOverrides, "template-overrides", false, "Generate page templates too (parse-tested and smoke-rendered)")
	generateCmd.Flags().StringVar(&opts.edition, "edition", "", "Edition sources to ground on: blog or storefront (default: server edition)")
	generateCmd.Flags().StringVar(&opts.timeout, "timeout", defaultGenerateTimeout, "Overall generation deadline (Go duration, e.g. 5m; 0 disables)")

	return generateCmd
}

func newThemeListCmd(apiKey, baseURL, output *string) *cobra.Command {
	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List installed themes",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runThemeList(cmd, themeNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return listCmd
}

func newThemeDownloadCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts themeDownloadOptions

	downloadCmd := &cobra.Command{
		Use:   "download <name>",
		Short: "Download an installed theme as a zip",
		Args:  cobra.ExactArgs(1),
		Long: `Download an installed theme as <name>.zip for manual THEME_DIR install.

The archive downloads to a .part file first and is renamed into place only
on success, so an interrupted run never leaves a truncated zip masquerading
as a complete theme.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runThemeDownload(cmd, args[0], opts)
		},
	}

	downloadCmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the theme zip")
	downloadCmd.Flags().StringVar(&opts.timeout, "timeout", defaultDownloadTimeout, "Overall download deadline (Go duration, e.g. 30m, 2h; 0 disables)")
	downloadCmd.Flags().StringVar(&opts.maxDownloadSize, "max-download-size", defaultMaxDownloadSize, "Download size cap (bytes, or K/M/G suffixes like 500MB; 0 disables)")

	return downloadCmd
}

func newThemeActivateCmd(apiKey, baseURL, output *string) *cobra.Command {
	activateCmd := &cobra.Command{
		Use:           "activate <name>",
		Short:         "Activate an installed theme",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemeActivate(cmd, args[0], themeNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return activateCmd
}

func newThemeDeactivateCmd(apiKey, baseURL, output *string) *cobra.Command {
	deactivateCmd := &cobra.Command{
		Use:           "deactivate",
		Short:         "Deactivate back to the embedded default theme",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runThemeDeactivate(cmd, themeNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return deactivateCmd
}

func newThemeDeleteCmd(apiKey, baseURL, output *string) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:           "delete <name>",
		Short:         "Delete an installed theme",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemeDelete(cmd, args[0], themeNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return deleteCmd
}

func parseGenerateTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		raw = defaultGenerateTimeout
	}
	dur, err := time.ParseDuration(raw)
	if err != nil || dur < 0 {
		return 0, &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --timeout %q (want a Go duration like 5m, or 0 to disable)", raw),
		}
	}
	return dur, nil
}

func runThemeGenerate(cmd *cobra.Command, opts themeGenerateOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	description := strings.TrimSpace(opts.description)
	if description == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --description is required",
		}
	}
	timeout, err := parseGenerateTimeout(opts.timeout)
	if err != nil {
		return err
	}

	key, url, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	c, err := client.New(url, key)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	data, meta, err := c.GenerateTheme(ctx, client.GenerateThemeRequest{
		Description:       description,
		Style:             strings.TrimSpace(opts.style),
		TemplateOverrides: opts.templateOverrides,
		Edition:           strings.TrimSpace(opts.edition),
	})
	if err != nil {
		if apiErr, ok := errors.AsType[*client.APIError](err); ok {
			if reason := firstFindingMessage(apiErr.Details); reason != "" {
				return &exitError{code: client.ExitCode(err), msg: fmt.Sprintf("Generation failed: %s", reason)}
			}
		}
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printThemeGenerated(cmd.OutOrStdout(), opts.output, data, meta)
}

func printThemeGenerated(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload themeGenerateData
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode generate response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Generated theme: %s\n", payload.Theme.Name); err != nil {
		return err
	}
	if payload.Theme.Rationale != "" {
		if _, err := fmt.Fprintf(w, "Rationale: %s\n", payload.Theme.Rationale); err != nil {
			return err
		}
	}
	for _, finding := range payload.Validation.Findings {
		if _, err := fmt.Fprintf(w, "  %s: %s\n", finding.Severity, finding.Message); err != nil {
			return err
		}
	}
	return nil
}

func runThemeList(cmd *cobra.Command, opts themeNameOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	key, url, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	c, err := client.New(url, key)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	data, meta, err := c.ThemesList(ctx)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printThemesList(cmd.OutOrStdout(), opts.output, data, meta)
}

func printThemesList(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload struct {
		Themes []installedThemeSummary `json:"themes"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode themes response: %w", err)
	}
	for _, theme := range payload.Themes {
		active := "inactive"
		if theme.Active {
			active = "active"
		}
		if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n", theme.Name, theme.Edition, active, theme.Source, theme.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func runThemeDownload(cmd *cobra.Command, name string, opts themeDownloadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: theme name is required",
		}
	}

	key, url, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	c, err := client.New(url, key)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	filePath := filepath.Join(opts.outputDir, name+".zip")
	partPath := filePath + ".part"
	f, err := os.Create(partPath)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: fmt.Sprintf("failed to create file: %s", err)}
	}
	defer func() { _ = f.Close() }()

	downloadOpts, derr := parseDownloadOptions(opts.timeout, opts.maxDownloadSize)
	if derr != nil {
		_ = f.Close()
		_ = os.Remove(partPath)
		return derr
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	if err := c.ThemeDownload(ctx, name, f, downloadOpts); err != nil {
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

	if opts.output == "json" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `{"message":%q,"path":%q}`+"\n", "theme downloaded", filePath)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Theme downloaded: %s\n", filePath)
	}
	return nil
}

func runThemeActivate(cmd *cobra.Command, name string, opts themeNameOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: theme name is required",
		}
	}

	key, url, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	c, err := client.New(url, key)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	data, meta, err := c.ThemeActivate(ctx, name)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printThemeActivation(cmd.OutOrStdout(), opts.output, data, meta, true)
}

func runThemeDeactivate(cmd *cobra.Command, opts themeNameOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}

	key, url, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	c, err := client.New(url, key)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	data, meta, err := c.ThemeDeactivate(ctx)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printThemeActivation(cmd.OutOrStdout(), opts.output, data, meta, false)
}

func printThemeActivation(w io.Writer, mode string, data, meta json.RawMessage, activating bool) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload themeActivationData
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode activation response: %w", err)
	}
	if payload.RequiresRestart {
		_, err := fmt.Fprintln(w, "Theme activated. Restart to apply.")
		return err
	}
	if activating {
		_, err := fmt.Fprintln(w, "Theme applied.")
		return err
	}
	_, err := fmt.Fprintln(w, "Embedded theme applied.")
	return err
}

func runThemeDelete(cmd *cobra.Command, name string, opts themeNameOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: theme name is required",
		}
	}

	key, url, err := resolveCredentials(opts.apiKey, opts.baseURL)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	c, err := client.New(url, key)
	if err != nil {
		return &exitError{code: client.ExitGeneric, msg: err.Error()}
	}

	ctx, stop := signalContext(cmd.Context())
	defer stop()
	data, meta, err := c.ThemeDelete(ctx, name)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "json" {
		return printEnvelope(cmd.OutOrStdout(), data, meta)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted theme: %s\n", name)
	return err
}
