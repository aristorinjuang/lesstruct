package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// moduleGenerateArgs carries the `module_generate` payload. Every field maps
// one-to-one onto the /api/v1/modgen/generate body; description is required
// and an empty surfaces list lets the model infer them from the brief.
type moduleGenerateArgs struct {
	Description string   `json:"description" jsonschema:"natural-language module brief (required, max 5,000 chars)"`
	Surfaces    []string `json:"surfaces,omitempty" jsonschema:"kernel surfaces the module implements, e.g. [routes, migrations] (empty infers from the brief)"`
}

// moduleDeleteArgs carries the `module_delete` payload. Deletion is refused
// without confirm:true, mirroring the destructive user decisions.
type moduleDeleteArgs struct {
	Name    string `json:"name" jsonschema:"installed project name"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"required true: deletion is final"`
}

// moduleApplyArgs carries the `module_apply` payload. Replace restages over
// an applied tree the registry owns. Confirmation is always required:
// staging mutates the checkout and triggers compiler execution, so no
// agent call proceeds without an explicit decision, replace or not.
type moduleApplyArgs struct {
	Name    string `json:"name" jsonschema:"installed project name"`
	Replace bool   `json:"replace,omitempty" jsonschema:"overwrite an applied tree the registry owns (default false)"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"required true: staging mutates the checkout and runs the compiler"`
}

// moduleUnapplyArgs carries the `module_unapply` payload. Removal is refused
// without confirm:true, mirroring the destructive user decisions.
type moduleUnapplyArgs struct {
	Name    string `json:"name" jsonschema:"installed applied module name"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"required true: removal deletes the applied tree"`
}

// moduleGenerated is the curated `module_generate` success projection. The raw
// payload carries the full files map and MCP hosts truncate long tool results,
// so the agent gets paths instead of bodies — full contents travel via
// `lesstruct-cli module download` (a zip archive), which the MCP surface
// deliberately omits as binary.
type moduleGenerated struct {
	Name      string   `json:"name"`
	Surfaces  []string `json:"surfaces,omitempty"`
	Rationale string   `json:"rationale,omitempty"`
	Valid     bool     `json:"valid"`
	Findings  []string `json:"findings,omitempty"`
	FileNames []string `json:"fileNames"`
}

// registerModuleTools adds the AI module generation tools to ms. Generation can
// take minutes (the repair loop retries up to twice); every result stages as
// an inert source project under modules-gen/ — Tier 1 modules link at compile
// time, so the operator copies the tree into a checkout and rebuilds (the
// download ships INSTALL.md with the custom-edition wiring and rebuild
// steps). All handlers close over the shared /api/v1 client.
func registerModuleTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "module_generate",
		Description: "Generate a validated Tier 1 Go module project from a natural-language brief (admin only, can take minutes). Stages as inert source under modules-gen/ — download the zip via lesstruct-cli, copy into modules/<name>/, wire a custom edition, and rebuild. Returns name, rationale, validation, and file paths (not contents).",
	}, makeModuleGenerateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "module_projects",
		Description: "List generated module projects with surfaces and file counts.",
	}, makeModuleProjectsHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "module_apply",
		Description: "Stage a generated module project into the server's source checkout and verify it (admin only, can take minutes). Requires MODULE_APPLY_ENABLED plus a Go toolchain and a source checkout; verification failures keep the applied tree and report per-step outcomes. Returns applied, verified, steps, and the edition's module list.",
	}, makeModuleApplyHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "module_unapply",
		Description: "Remove an applied module from the server's source checkout (admin only, requires confirm:true). Regenerates the custom edition without the module, drops its registrations, deletes the tree, and rebuilds the custom binary to verify. Returns removed, verified, and moduleDirRemoved.",
	}, makeModuleUnapplyHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "module_delete",
		Description: "Delete a generated module project (admin only, requires confirm:true).",
	}, makeModuleDeleteHandler(c))
}

// makeModuleGenerateHandler returns the `module_generate` handler.
func makeModuleGenerateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, moduleGenerateArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args moduleGenerateArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.Description == "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: description is required")), nil, nil
		}
		data, _, err := c.GenerateModule(ctx, client.GenerateModuleRequest{
			Description: args.Description,
			Surfaces:    args.Surfaces,
		})
		if err != nil {
			var apiErr *client.APIError
			if errors.As(err, &apiErr) {
				if reason := firstErrorFinding(apiErr.Details); reason != "" {
					return toolError(fmt.Errorf("GENERATION_FAILED: %s", reason)), nil, nil
				}
			}
			return toolError(err), nil, nil
		}
		var payload struct {
			Project struct {
				Name      string            `json:"name"`
				Surfaces  []string          `json:"surfaces"`
				Rationale string            `json:"rationale"`
				Files     map[string]string `json:"files"`
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
		if err := json.Unmarshal(data, &payload); err != nil {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned an undecodable project: %w", err)), nil, nil
		}
		generated := moduleGenerated{
			Name:      payload.Project.Name,
			Surfaces:  payload.Project.Surfaces,
			Rationale: payload.Project.Rationale,
			Valid:     payload.Validation.Valid,
		}
		for _, finding := range payload.Validation.Findings {
			generated.Findings = append(generated.Findings, finding.Severity+": "+finding.Message)
		}
		for name := range payload.Project.Files {
			generated.FileNames = append(generated.FileNames, name)
		}
		slices.Sort(generated.FileNames)
		return toolJSON(generated), nil, nil
	}
}

// makeModuleProjectsHandler returns the `module_projects` handler.
func makeModuleProjectsHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.ModuleProjectsList(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeModuleUnapplyHandler returns the `module_unapply` handler.
func makeModuleUnapplyHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, moduleUnapplyArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args moduleUnapplyArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return toolError(fmt.Errorf("INVALID_NAME: project name is required")), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required for this destructive decision")), nil, nil
		}
		data, meta, err := c.ModuleUnapply(ctx, args.Name, true)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeModuleDeleteHandler returns the `module_delete` handler.
func makeModuleDeleteHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, moduleDeleteArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args moduleDeleteArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return toolError(fmt.Errorf("INVALID_NAME: project name is required")), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required for this destructive decision")), nil, nil
		}
		data, meta, err := c.ModuleDelete(ctx, args.Name)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeModuleApplyHandler returns the `module_apply` handler.
func makeModuleApplyHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, moduleApplyArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args moduleApplyArgs,
	) (*mcp.CallToolResult, any, error) {
		if args.Name == "" {
			return toolError(fmt.Errorf("INVALID_NAME: project name is required")), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required; module_apply mutates the server checkout and runs the compiler")), nil, nil
		}
		data, meta, err := c.ModuleApply(ctx, args.Name, client.ModuleApplyRequest{
			Replace: args.Replace,
			Confirm: true,
		})
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}
