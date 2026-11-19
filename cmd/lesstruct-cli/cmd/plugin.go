package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

// pluginNamePattern mirrors the server's project slug grammar so malformed
// names fail fast client-side instead of corrupting request paths or
// escaping the download directory.
var pluginNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,49}$`)

// validatePluginName trims and checks a project name against the slug
// grammar, returning the clean name or a usage error.
func validatePluginName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: plugin name is required",
		}
	}
	if !pluginNamePattern.MatchString(name) {
		return "", &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid plugin name %q (lowercase letters, digits, dashes)", raw),
		}
	}
	return name, nil
}

// pluginGenerateData is the cmd-layer projection of the generate response.
type pluginGenerateData struct {
	Project struct {
		Name      string `json:"name"`
		Language  string `json:"language"`
		Rationale string `json:"rationale"`
	} `json:"project"`
	Validation struct {
		Valid    bool `json:"valid"`
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"findings"`
	} `json:"validation"`
}

// installedPluginSummary is the cmd-layer projection of an installed project.
type installedPluginSummary struct {
	Name      string   `json:"name"`
	Language  string   `json:"language"`
	Hooks     []string `json:"hooks"`
	CreatedAt string   `json:"createdAt"`
	Source    string   `json:"source"`
}

// pluginGenerateOptions bundles the resolved flag values for plugin generate.
type pluginGenerateOptions struct {
	description  string
	language     string
	hooks        string
	capabilities string
	timeout      string
	apiKey       string
	baseURL      string
	output       string
}

// pluginNameOptions bundles the resolved flag values for single-project verbs.
type pluginNameOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// pluginDownloadOptions bundles the resolved flag values for plugin download.
type pluginDownloadOptions struct {
	outputDir       string
	timeout         string
	maxDownloadSize string
	apiKey          string
	baseURL         string
	output          string
}

// newPluginCmd builds the `plugin` command tree.
func newPluginCmd(apiKey, baseURL, output *string) *cobra.Command {
	plugin := &cobra.Command{
		Use:           "plugin",
		Short:         "Generate and manage AI plugin projects over /api/v1/plugingen",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	plugin.AddCommand(newPluginGenerateCmd(apiKey, baseURL, output))
	plugin.AddCommand(newPluginListCmd(apiKey, baseURL, output))
	plugin.AddCommand(newPluginDownloadCmd(apiKey, baseURL, output))
	plugin.AddCommand(newPluginBuildCmd(apiKey, baseURL, output))
	plugin.AddCommand(newPluginStatusCmd(apiKey, baseURL, output))
	plugin.AddCommand(newPluginDeleteCmd(apiKey, baseURL, output))

	return plugin
}

func splitCSV(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := strings.TrimSpace(part); cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

func newPluginGenerateCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts pluginGenerateOptions

	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a plugin project from a natural-language brief",
		Long: `Generate a validated WASM plugin project from a natural-language brief.

The generated project installs as source under plugins/src. Compile it with
TinyGo (tinygo build -o <name>.wasm -target=wasi main.go) and drop the .wasm
into plugins/.

The run can take minutes (the repair loop retries up to twice): --timeout
(default 5m) caps the whole call, 0 disables.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runPluginGenerate(cmd, opts)
		},
	}

	generateCmd.Flags().StringVar(&opts.description, "description", "", "Natural-language plugin brief (required, max 5,000 chars)")
	generateCmd.Flags().StringVar(&opts.language, "language", "go", "Plugin language (go only in v1)")
	generateCmd.Flags().StringVar(&opts.hooks, "hooks", "", "Comma-separated hook suffixes (required, e.g. before_save,after_create)")
	generateCmd.Flags().StringVar(&opts.capabilities, "capabilities", "", "Comma-separated host functions (e.g. http_get,log_info)")
	generateCmd.Flags().StringVar(&opts.timeout, "timeout", defaultGenerateTimeout, "Overall generation deadline (Go duration, e.g. 5m; 0 disables)")

	return generateCmd
}

func newPluginListCmd(apiKey, baseURL, output *string) *cobra.Command {
	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List generated plugin projects",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginList(cmd, pluginNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return listCmd
}

func newPluginDownloadCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts pluginDownloadOptions

	downloadCmd := &cobra.Command{
		Use:           "download <name>",
		Short:         "Download a generated plugin project as a zip",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runPluginDownload(cmd, args[0], opts)
		},
	}

	downloadCmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the project zip")
	downloadCmd.Flags().StringVar(&opts.timeout, "timeout", defaultDownloadTimeout, "Overall download deadline (Go duration, e.g. 30m, 2h; 0 disables)")
	downloadCmd.Flags().StringVar(&opts.maxDownloadSize, "max-download-size", defaultMaxDownloadSize, "Download size cap (bytes, or K/M/G suffixes like 500MB; 0 disables)")

	return downloadCmd
}

func newPluginStatusCmd(apiKey, baseURL, output *string) *cobra.Command {
	statusCmd := &cobra.Command{
		Use:           "status",
		Short:         "List loaded plugins",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginStatus(cmd, pluginNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return statusCmd
}

func newPluginBuildCmd(apiKey, baseURL, output *string) *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build <name>",
		Short: "Compile a plugin project to WASM and install it into plugins/",
		Long: `Compile a generated plugin project on the server with TinyGo and
install the .wasm plus the synthesized capability manifest into plugins/.

In DEV_MODE the watcher hot-reloads it; otherwise restart the server to
apply it. Requires PLUGIN_BUILD_ENABLED=true and a TinyGo binary on the
server (TINYGO_PATH); without one the call fails with 503
build_unavailable.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginBuild(cmd, args[0], pluginNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return buildCmd
}

func newPluginDeleteCmd(apiKey, baseURL, output *string) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:           "delete <name>",
		Short:         "Delete a generated plugin project",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginDelete(cmd, args[0], pluginNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return deleteCmd
}

func runPluginGenerate(cmd *cobra.Command, opts pluginGenerateOptions) error {
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
	hooks := splitCSV(opts.hooks)
	if len(hooks) == 0 {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --hooks is required (e.g. --hooks before_save,after_create)",
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
	data, meta, err := c.GeneratePlugin(ctx, client.GeneratePluginRequest{
		Description:  description,
		Language:     strings.TrimSpace(opts.language),
		Hooks:        hooks,
		Capabilities: splitCSV(opts.capabilities),
	})
	if err != nil {
		if apiErr, ok := errors.AsType[*client.APIError](err); ok {
			if reason := firstFindingMessage(apiErr.Details); reason != "" {
				return &exitError{code: client.ExitCode(err), msg: fmt.Sprintf("Generation failed: %s", reason)}
			}
		}
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printPluginGenerated(cmd.OutOrStdout(), opts.output, data, meta)
}

func printPluginGenerated(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload pluginGenerateData
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode generate response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Generated plugin: %s\n", payload.Project.Name); err != nil {
		return err
	}
	if payload.Project.Rationale != "" {
		if _, err := fmt.Fprintf(w, "Rationale: %s\n", payload.Project.Rationale); err != nil {
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

func runPluginList(cmd *cobra.Command, opts pluginNameOptions) error {
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
	data, meta, err := c.PluginProjectsList(ctx)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printPluginList(cmd.OutOrStdout(), opts.output, data, meta)
}

func printPluginList(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload struct {
		Projects []installedPluginSummary `json:"projects"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode projects response: %w", err)
	}
	for _, project := range payload.Projects {
		if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n", project.Name, project.Language, strings.Join(project.Hooks, ","), project.Source, project.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func runPluginDownload(cmd *cobra.Command, name string, opts pluginDownloadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name, err := validatePluginName(name)
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
	if err := c.PluginDownload(ctx, name, f, downloadOpts); err != nil {
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
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `{"message":%q,"path":%q}`+"\n", "plugin downloaded", filePath)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Plugin downloaded: %s\n", filePath)
	}
	return nil
}

func runPluginStatus(cmd *cobra.Command, opts pluginNameOptions) error {
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
	data, meta, err := c.PluginsStatus(ctx)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "json" {
		return printEnvelope(cmd.OutOrStdout(), data, meta)
	}
	var payload struct {
		Plugins []map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode status response: %w", err)
	}
	for _, plugin := range payload.Plugins {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%v\n", plugin)
	}
	return nil
}

func runPluginDelete(cmd *cobra.Command, name string, opts pluginNameOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name, err := validatePluginName(name)
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
	data, meta, err := c.PluginDelete(ctx, name)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "json" {
		return printEnvelope(cmd.OutOrStdout(), data, meta)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted plugin: %s\n", name)
	return err
}

func runPluginBuild(cmd *cobra.Command, name string, opts pluginNameOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name, err := validatePluginName(name)
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
	data, meta, err := c.PluginBuild(ctx, name)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printPluginBuild(cmd.OutOrStdout(), opts.output, name, data, meta)
}

func printPluginBuild(w io.Writer, mode, name string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload struct {
		Installed       bool `json:"installed"`
		RequiresRestart bool `json:"requiresRestart"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode build response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Installed plugin: %s\n", name); err != nil {
		return err
	}
	if payload.RequiresRestart {
		_, err := fmt.Fprintln(w, "Restart the server to apply.")
		return err
	}
	return nil
}
