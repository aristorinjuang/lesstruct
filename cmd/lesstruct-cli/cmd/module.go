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
	"time"

	client "github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/spf13/cobra"
)

// moduleNamePattern mirrors the server's module name grammar so malformed
// names fail fast client-side instead of corrupting request paths or
// escaping the download directory. Module names stay hyphen-free so the
// package clause needs no rename.
var moduleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]{1,49}$`)

// validateModuleName trims and checks a project name against the module
// grammar, returning the clean name or a usage error.
func validateModuleName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: module name is required",
		}
	}
	if !moduleNamePattern.MatchString(name) {
		return "", &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid module name %q (lowercase alphanumerics, hyphen-free)", raw),
		}
	}
	return name, nil
}

// moduleGenerateData is the cmd-layer projection of the generate response.
type moduleGenerateData struct {
	Project struct {
		Name      string   `json:"name"`
		Surfaces  []string `json:"surfaces"`
		Rationale string   `json:"rationale"`
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

// installedModuleSummary is the cmd-layer projection of an installed project.
type installedModuleSummary struct {
	Name      string   `json:"name"`
	Surfaces  []string `json:"surfaces"`
	FileCount int      `json:"fileCount"`
	CreatedAt string   `json:"createdAt"`
	Source    string   `json:"source"`
}

// moduleGenerateOptions bundles the resolved flag values for module generate.
type moduleGenerateOptions struct {
	description string
	surfaces    string
	timeout     string
	apiKey      string
	baseURL     string
	output      string
}

// moduleApplyOptions bundles the resolved flag values for module apply.
type moduleApplyOptions struct {
	replace bool
	confirm bool
	timeout string
	apiKey  string
	baseURL string
	output  string
}

// moduleNameOptions bundles the resolved flag values for single-project verbs.
type moduleNameOptions struct {
	apiKey  string
	baseURL string
	output  string
}

// moduleDownloadOptions bundles the resolved flag values for module download.
type moduleDownloadOptions struct {
	outputDir       string
	timeout         string
	maxDownloadSize string
	apiKey          string
	baseURL         string
	output          string
}

// newModuleCmd builds the `module` command tree.
func newModuleCmd(apiKey, baseURL, output *string) *cobra.Command {
	module := &cobra.Command{
		Use:           "module",
		Short:         "Generate and manage AI module projects over /api/v1/modgen",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	module.AddCommand(newModuleGenerateCmd(apiKey, baseURL, output))
	module.AddCommand(newModuleApplyCmd(apiKey, baseURL, output))
	module.AddCommand(newModuleUnapplyCmd(apiKey, baseURL, output))
	module.AddCommand(newModuleListCmd(apiKey, baseURL, output))
	module.AddCommand(newModuleDownloadCmd(apiKey, baseURL, output))
	module.AddCommand(newModuleDeleteCmd(apiKey, baseURL, output))

	return module
}

func newModuleGenerateCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts moduleGenerateOptions

	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a module project from a natural-language brief",
		Long: `Generate a validated Tier 1 Go module project from a natural-language brief.

The generated project stages as inert source under modules-gen/. Tier 1
modules link at compile time: the download ships an INSTALL.md with the
custom-edition wiring and rebuild steps — there is no runtime install.

The run can take minutes (the repair loop retries up to twice): --timeout
(default 5m) caps the whole call, 0 disables.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runModuleGenerate(cmd, opts)
		},
	}

	generateCmd.Flags().StringVar(&opts.description, "description", "", "Natural-language module brief (required, max 5,000 chars)")
	generateCmd.Flags().StringVar(&opts.surfaces, "surfaces", "", "Comma-separated kernel surfaces (e.g. routes,migrations; empty lets the model infer them)")
	generateCmd.Flags().StringVar(&opts.timeout, "timeout", defaultModuleGenerateTimeout, "Overall generation deadline (Go duration, e.g. 6m; 0 disables)")

	return generateCmd
}

func newModuleApplyCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts moduleApplyOptions

	applyCmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Stage a module project into the source checkout and verify it",
		Long: `Stage a generated module project into the server's source checkout:
copy the tree to modules/<name>/, register it in go.work and SUBMODULES,
regenerate the custom edition, and verify (module tests plus the
custom-binary build).

Requires MODULE_APPLY_ENABLED=true on the server plus a Go toolchain and
a source checkout at the apply root; otherwise the call fails with 503
apply_unavailable. Verification failures keep the applied tree and report
per-step outcomes — rebuild ./cmd/lesstruct-custom and deploy it yourself.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runModuleApply(cmd, args[0], opts)
		},
	}

	applyCmd.Flags().BoolVar(&opts.replace, "replace", false, "Overwrite an applied tree the registry owns (requires --confirm)")
	applyCmd.Flags().BoolVar(&opts.confirm, "confirm", false, "Confirm replacing an applied tree")
	applyCmd.Flags().StringVar(&opts.timeout, "timeout", "16m", "Overall apply deadline (Go duration, e.g. 16m; 0 disables)")

	return applyCmd
}

func newModuleListCmd(apiKey, baseURL, output *string) *cobra.Command {
	listCmd := &cobra.Command{
		Use:           "list",
		Short:         "List generated module projects",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModuleList(cmd, moduleNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return listCmd
}

func newModuleDownloadCmd(apiKey, baseURL, output *string) *cobra.Command {
	var opts moduleDownloadOptions

	downloadCmd := &cobra.Command{
		Use:           "download <name>",
		Short:         "Download a generated module project as a zip",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.apiKey = *apiKey
			opts.baseURL = *baseURL
			opts.output = *output
			return runModuleDownload(cmd, args[0], opts)
		},
	}

	downloadCmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "Directory to write the project zip")
	downloadCmd.Flags().StringVar(&opts.timeout, "timeout", defaultDownloadTimeout, "Overall download deadline (Go duration, e.g. 30m, 2h; 0 disables)")
	downloadCmd.Flags().StringVar(&opts.maxDownloadSize, "max-download-size", defaultMaxDownloadSize, "Download size cap (bytes, or K/M/G suffixes like 500MB; 0 disables)")

	return downloadCmd
}

func newModuleDeleteCmd(apiKey, baseURL, output *string) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:           "delete <name>",
		Short:         "Delete a generated module project",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleDelete(cmd, args[0], moduleNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			})
		},
	}

	return deleteCmd
}

func newModuleUnapplyCmd(apiKey, baseURL, output *string) *cobra.Command {
	var confirm bool

	unapplyCmd := &cobra.Command{
		Use:   "unapply <name>",
		Short: "Remove an applied module from the source checkout",
		Long: `Remove a module the checkout registry holds: the custom edition is
regenerated without it, the go.work and SUBMODULES registrations are
dropped, the module tree is deleted, and the custom binary is rebuilt to
verify. Tracked operator edits inside the tree refuse the removal;
re-apply restores a removed module. Requires --confirm.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleUnapply(cmd, args[0], moduleNameOptions{
				apiKey:  *apiKey,
				baseURL: *baseURL,
				output:  *output,
			}, confirm)
		},
	}

	unapplyCmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm removing the applied module (required)")

	return unapplyCmd
}

func runModuleGenerate(cmd *cobra.Command, opts moduleGenerateOptions) error {
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
	surfaces := splitCSV(opts.surfaces)
	timeout, err := parseModuleGenerateTimeout(opts.timeout)
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
	data, meta, err := c.GenerateModule(ctx, client.GenerateModuleRequest{
		Description: description,
		Surfaces:    surfaces,
	})
	if err != nil {
		if apiErr, ok := errors.AsType[*client.APIError](err); ok {
			if reason := firstFindingMessage(apiErr.Details); reason != "" {
				return &exitError{code: client.ExitCode(err), msg: fmt.Sprintf("Generation failed: %s", reason)}
			}
		}
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printModuleGenerated(cmd.OutOrStdout(), opts.output, data, meta)
}

func printModuleGenerated(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload moduleGenerateData
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode generate response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Generated module: %s\n", payload.Project.Name); err != nil {
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

// defaultModuleGenerateTimeout caps one module generation run from the CLI
// with headroom over the server's 5-minute budget, so the server's error
// response arrives before the client cuts the call.
const defaultModuleGenerateTimeout = "6m"

// defaultApplyTimeout caps one module apply end to end: module tests in
// both workspaces plus the custom-binary build share it, with headroom
// over the server's 15-minute budget.
const defaultApplyTimeout = "16m"

// firstLine keeps the head of a multi-line step output for the text
// projection; full tails ride --output json.
func firstLine(output string) string {
	if head, _, ok := strings.Cut(output, "\n"); ok {
		return head
	}
	return output
}

// moduleApplyData is the cmd-layer projection of the apply response.
type moduleApplyData struct {
	Project struct {
		Name string `json:"name"`
	} `json:"project"`
	Apply struct {
		Applied  bool `json:"applied"`
		Verified bool `json:"verified"`
		Steps    []struct {
			Name    string `json:"name"`
			Passed  bool   `json:"passed"`
			Skipped bool   `json:"skipped"`
			Output  string `json:"output"`
		} `json:"steps"`
		AppliedModules  []string `json:"appliedModules"`
		RequiresRestart bool     `json:"requiresRestart"`
	} `json:"apply"`
}

// parseModuleGenerateTimeout parses the module generate deadline flag,
// defaulting above the server's 5-minute budget.
func parseModuleGenerateTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		raw = defaultModuleGenerateTimeout
	}
	dur, err := time.ParseDuration(raw)
	if err != nil || dur < 0 {
		return 0, &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --timeout %q (want a Go duration like 6m, or 0 to disable)", raw),
		}
	}
	return dur, nil
}

// parseApplyTimeout parses the apply deadline flag, defaulting to the
// server-side verification budget plus headroom.
func parseApplyTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		raw = defaultApplyTimeout
	}
	dur, err := time.ParseDuration(raw)
	if err != nil || dur < 0 {
		return 0, &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --timeout %q (want a Go duration like 16m, or 0 to disable)", raw),
		}
	}
	return dur, nil
}

func runModuleApply(cmd *cobra.Command, name string, opts moduleApplyOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name, err := validateModuleName(name)
	if err != nil {
		return err
	}
	if opts.replace && !opts.confirm {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --replace requires --confirm",
		}
	}
	timeout, err := parseApplyTimeout(opts.timeout)
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
	data, meta, err := c.ModuleApply(ctx, name, client.ModuleApplyRequest{
		Replace: opts.replace,
		Confirm: opts.confirm,
	})
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printModuleApply(cmd.OutOrStdout(), opts.output, name, data, meta)
}

func printModuleApply(w io.Writer, mode, name string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload moduleApplyData
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode apply response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Applied module: %s\n", name); err != nil {
		return err
	}
	if !payload.Apply.Verified {
		if _, err := fmt.Fprintln(w, "Verification failed — the tree is kept; inspect the steps:"); err != nil {
			return err
		}
	}
	for _, step := range payload.Apply.Steps {
		status := "pass"
		if step.Skipped {
			status = "skipped"
		} else if !step.Passed {
			status = "FAIL"
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", step.Name, status); err != nil {
			return err
		}
		if step.Output != "" {
			if _, err := fmt.Fprintf(w, "    %s\n", firstLine(step.Output)); err != nil {
				return err
			}
		}
	}
	if payload.Apply.RequiresRestart {
		_, err := fmt.Fprintln(w, "Rebuild ./cmd/lesstruct-custom and redeploy to apply.")
		return err
	}
	return nil
}

func runModuleList(cmd *cobra.Command, opts moduleNameOptions) error {
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
	data, meta, err := c.ModuleProjectsList(ctx)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printModuleList(cmd.OutOrStdout(), opts.output, data, meta)
}

func printModuleList(w io.Writer, mode string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload struct {
		Projects []installedModuleSummary `json:"projects"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode projects response: %w", err)
	}
	for _, project := range payload.Projects {
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", project.Name, strings.Join(project.Surfaces, ","), project.Source, project.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func runModuleDownload(cmd *cobra.Command, name string, opts moduleDownloadOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name, err := validateModuleName(name)
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
	if err := c.ModuleDownload(ctx, name, f, downloadOpts); err != nil {
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
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `{"message":%q,"path":%q}`+"\n", "module downloaded", filePath)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Module downloaded: %s\n", filePath)
	}
	return nil
}

func runModuleDelete(cmd *cobra.Command, name string, opts moduleNameOptions) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name, err := validateModuleName(name)
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
	data, meta, err := c.ModuleDelete(ctx, name)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	if opts.output == "json" {
		return printEnvelope(cmd.OutOrStdout(), data, meta)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted module: %s\n", name)
	return err
}

// moduleUnapplyData is the cmd-layer projection of the unapply response.
type moduleUnapplyData struct {
	Unapply struct {
		Removed  bool `json:"removed"`
		Verified bool `json:"verified"`
		Steps    []struct {
			Name    string `json:"name"`
			Passed  bool   `json:"passed"`
			Skipped bool   `json:"skipped"`
			Output  string `json:"output"`
		} `json:"steps"`
		ModuleDirRemoved bool `json:"moduleDirRemoved"`
	} `json:"unapply"`
}

func runModuleUnapply(cmd *cobra.Command, name string, opts moduleNameOptions, confirm bool) error {
	if err := validateOutput(opts.output); err != nil {
		return err
	}
	name, err := validateModuleName(name)
	if err != nil {
		return err
	}
	if !confirm {
		return &exitError{
			code: client.ExitUsage,
			msg:  "lesstruct-cli: --confirm is required (removal deletes the applied tree)",
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
	data, meta, err := c.ModuleUnapply(ctx, name, true)
	if err != nil {
		return &exitError{code: client.ExitCode(err), msg: apiErrorMessage(err)}
	}

	return printModuleUnapply(cmd.OutOrStdout(), opts.output, name, data, meta)
}

func printModuleUnapply(w io.Writer, mode, name string, data, meta json.RawMessage) error {
	if mode == "json" {
		return printEnvelope(w, data, meta)
	}
	var payload moduleUnapplyData
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode unapply response: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Removed module: %s\n", name); err != nil {
		return err
	}
	if !payload.Unapply.Verified {
		if _, err := fmt.Fprintln(w, "Verification failed — the removal stays; inspect the steps:"); err != nil {
			return err
		}
	}
	for _, step := range payload.Unapply.Steps {
		status := "pass"
		if step.Skipped {
			status = "skipped"
		} else if !step.Passed {
			status = "FAIL"
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", step.Name, status); err != nil {
			return err
		}
	}
	return nil
}
