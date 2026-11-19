package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// pluginGenerateArgs carries the `plugin_generate` payload. Every field maps
// one-to-one onto the /api/v1/plugingen/generate body; description and hooks
// are required.
type pluginGenerateArgs struct {
	Description  string   `json:"description" jsonschema:"natural-language plugin brief (required, max 5,000 chars)"`
	Language     string   `json:"language,omitempty" jsonschema:"plugin language: go (default, TinyGo WASI)"`
	Hooks        []string `json:"hooks" jsonschema:"hook suffixes to implement, e.g. [before_save, after_create]"`
	Capabilities []string `json:"capabilities,omitempty" jsonschema:"host functions needed, e.g. [http_get, log_info]"`
}

// pluginDeleteArgs carries the `plugin_delete` payload. Deletion is refused
// without confirm:true, mirroring the destructive user decisions.
type pluginDeleteArgs struct {
	Name    string `json:"name" jsonschema:"installed project name"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"required true: deletion is final"`
}

// pluginBuildArgs carries the `plugin_build` payload.
type pluginBuildArgs struct {
	Name string `json:"name" jsonschema:"installed project name"`
}

// pluginGenerated is the curated `plugin_generate` success projection. The raw
// payload carries the full files map and MCP hosts truncate long tool results,
// so the agent gets paths instead of bodies — full contents travel via
// `lesstruct-cli plugin download` (a zip archive), which the MCP surface
// deliberately omits as binary.
type pluginGenerated struct {
	Name          string   `json:"name"`
	Language      string   `json:"language"`
	Rationale     string   `json:"rationale,omitempty"`
	Valid         bool     `json:"valid"`
	Findings      []string `json:"findings,omitempty"`
	FileNames     []string `json:"fileNames"`
	Hooks         []string `json:"hooks,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	HTTPPatterns  []string `json:"httpPatterns,omitempty"`
	DBPermissions []string `json:"dbPermissions,omitempty"`
}

// registerPluginTools adds the AI plugin generation tools to ms. Generation can
// take minutes (the repair loop retries up to twice); every result installs
// as a source project under plugins/src — compile with TinyGo (or
// plugin_build on a build-enabled server) and drop the .wasm into plugins/.
// All handlers close over the shared /api/v1 client.
func registerPluginTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "plugin_generate",
		Description: "Generate a validated WASM plugin project from a natural-language brief (admin only, can take minutes). Installs as a source project under plugins/src — compile with TinyGo, build on a build-enabled server via plugin_build, or drop the .wasm into plugins/. Returns name, rationale, validation, grants, and file paths (not contents).",
	}, makePluginGenerateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "plugin_build",
		Description: "Compile a generated plugin project to WASM and install it into plugins/ (admin only, can take minutes, requires a build-enabled server). Returns installed and requiresRestart.",
	}, makePluginBuildHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "plugin_projects",
		Description: "List generated plugin projects with language and hooks.",
	}, makePluginProjectsHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "plugin_status",
		Description: "List loaded plugins with hook registrations (read-only inspection).",
	}, makePluginStatusHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "plugin_delete",
		Description: "Delete a generated plugin project (admin only, requires confirm:true).",
	}, makePluginDeleteHandler(c))
}

// makePluginGenerateHandler returns the `plugin_generate` handler.
func makePluginGenerateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, pluginGenerateArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args pluginGenerateArgs,
	) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Description) == "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: description is required")), nil, nil
		}
		if len(args.Hooks) == 0 {
			return toolError(fmt.Errorf("VALIDATION_ERROR: at least one hook is required")), nil, nil
		}
		data, _, err := c.GeneratePlugin(ctx, client.GeneratePluginRequest{
			Description:  args.Description,
			Language:     args.Language,
			Hooks:        args.Hooks,
			Capabilities: args.Capabilities,
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
				Name          string            `json:"name"`
				Language      string            `json:"language"`
				Rationale     string            `json:"rationale"`
				Hooks         []string          `json:"hooks"`
				Capabilities  []string          `json:"capabilities"`
				HTTPPatterns  []string          `json:"httpPatterns"`
				DBPermissions []string          `json:"dbPermissions"`
				Files         map[string]string `json:"files"`
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
		generated := pluginGenerated{
			Name:          payload.Project.Name,
			Language:      payload.Project.Language,
			Rationale:     payload.Project.Rationale,
			Valid:         payload.Validation.Valid,
			Hooks:         payload.Project.Hooks,
			Capabilities:  payload.Project.Capabilities,
			HTTPPatterns:  payload.Project.HTTPPatterns,
			DBPermissions: payload.Project.DBPermissions,
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

// makePluginBuildHandler returns the `plugin_build` handler.
func makePluginBuildHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, pluginBuildArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args pluginBuildArgs,
	) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Name) == "" {
			return toolError(fmt.Errorf("INVALID_NAME: project name is required")), nil, nil
		}
		data, meta, err := c.PluginBuild(ctx, strings.TrimSpace(args.Name))
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makePluginProjectsHandler returns the `plugin_projects` handler.
func makePluginProjectsHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.PluginProjectsList(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makePluginStatusHandler returns the `plugin_status` handler.
func makePluginStatusHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.PluginsStatus(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makePluginDeleteHandler returns the `plugin_delete` handler.
func makePluginDeleteHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, pluginDeleteArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args pluginDeleteArgs,
	) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Name) == "" {
			return toolError(fmt.Errorf("INVALID_NAME: project name is required")), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required for this destructive decision")), nil, nil
		}
		data, meta, err := c.PluginDelete(ctx, strings.TrimSpace(args.Name))
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}
