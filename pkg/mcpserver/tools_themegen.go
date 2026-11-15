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

// themeGenerateArgs carries the `theme_generate` payload. Every field maps
// one-to-one onto the /api/v1/themegen/generate body; description is the
// only required one.
type themeGenerateArgs struct {
	Description       string `json:"description" jsonschema:"natural-language theme brief (required, max 5,000 chars)"`
	Style             string `json:"style,omitempty" jsonschema:"style preset: auto (default), dark, light, minimal, playful, editorial, brutalist"`
	TemplateOverrides bool   `json:"templateOverrides,omitempty" jsonschema:"also generate page templates (parse-tested and smoke-rendered)"`
	Edition           string `json:"edition,omitempty" jsonschema:"edition sources to ground on: blog or storefront (default: server edition)"`
}

// themeNameArgs carries the single-theme tools' payload.
type themeNameArgs struct {
	Name string `json:"name" jsonschema:"installed theme name"`
}

// themeDeleteArgs carries the `theme_delete` payload. Deletion is refused
// without confirm:true, mirroring the destructive user decisions.
type themeDeleteArgs struct {
	Name    string `json:"name" jsonschema:"installed theme name"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"required true: deletion is final"`
}

// themeGenerated is the curated `theme_generate` success projection. The raw
// payload carries the full files map (up to 2 MiB of CSS) and MCP hosts
// truncate long tool results, so the agent gets paths instead of bodies —
// file contents stay reachable via the lesstruct_api escape hatch.
type themeGenerated struct {
	Name      string   `json:"name"`
	Edition   string   `json:"edition"`
	Rationale string   `json:"rationale,omitempty"`
	Valid     bool     `json:"valid"`
	Findings  []string `json:"findings,omitempty"`
	FileNames []string `json:"fileNames"`
}

// registerThemeTools adds the AI theme generation tools to ms. Generation can
// take minutes (the repair loop retries up to twice); every result
// auto-installs inactive — activate to apply. All handlers close over the
// shared /api/v1 client.
func registerThemeTools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "theme_generate",
		Description: "Generate a validated theme from a natural-language brief (admin only, can take minutes). Stylesheet-only by default; templateOverrides also generates page templates. Auto-installs inactive — theme_activate applies it. Returns name, rationale, validation, and file paths (not contents).",
	}, makeThemeGenerateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "theme_list",
		Description: "List installed themes with active markers and edition.",
	}, makeThemeListHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "theme_activate",
		Description: "Activate an installed theme (admin only). Hot-applies when the edition supports it; otherwise reports requiresRestart.",
	}, makeThemeActivateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "theme_deactivate",
		Description: "Deactivate back to the embedded default theme (admin only).",
	}, makeThemeDeactivateHandler(c))
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "theme_delete",
		Description: "Delete an installed theme (admin only, requires confirm:true). Refuses while the theme is active.",
	}, makeThemeDeleteHandler(c))
}

// firstErrorFinding extracts the first error-severity finding message from a
// generation-failure details payload, mirroring the CLI failure line.
func firstErrorFinding(details json.RawMessage) string {
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

// makeThemeGenerateHandler returns the `theme_generate` handler.
func makeThemeGenerateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, themeGenerateArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args themeGenerateArgs,
	) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Description) == "" {
			return toolError(fmt.Errorf("VALIDATION_ERROR: description is required")), nil, nil
		}
		data, _, err := c.GenerateTheme(ctx, client.GenerateThemeRequest{
			Description:       args.Description,
			Style:             args.Style,
			TemplateOverrides: args.TemplateOverrides,
			Edition:           args.Edition,
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
			Theme struct {
				Name      string            `json:"name"`
				Edition   string            `json:"edition"`
				Rationale string            `json:"rationale"`
				Files     map[string]string `json:"files"`
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
		if err := json.Unmarshal(data, &payload); err != nil {
			return toolError(fmt.Errorf("INTERNAL_ERROR: server returned an undecodable theme: %w", err)), nil, nil
		}
		generated := themeGenerated{
			Name:      payload.Theme.Name,
			Edition:   payload.Theme.Edition,
			Rationale: payload.Theme.Rationale,
			Valid:     payload.Validation.Valid,
		}
		for _, finding := range payload.Validation.Findings {
			generated.Findings = append(generated.Findings, finding.Severity+": "+finding.Message)
		}
		for name := range payload.Theme.Files {
			generated.FileNames = append(generated.FileNames, name)
		}
		slices.Sort(generated.FileNames)
		return toolJSON(generated), nil, nil
	}
}

// makeThemeListHandler returns the `theme_list` handler.
func makeThemeListHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.ThemesList(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeThemeActivateHandler returns the `theme_activate` handler.
func makeThemeActivateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, themeNameArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args themeNameArgs,
	) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Name) == "" {
			return toolError(fmt.Errorf("INVALID_NAME: theme name is required")), nil, nil
		}
		data, meta, err := c.ThemeActivate(ctx, strings.TrimSpace(args.Name))
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeThemeDeactivateHandler returns the `theme_deactivate` handler.
func makeThemeDeactivateHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		data, meta, err := c.ThemeDeactivate(ctx)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// makeThemeDeleteHandler returns the `theme_delete` handler.
func makeThemeDeleteHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, themeDeleteArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args themeDeleteArgs,
	) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Name) == "" {
			return toolError(fmt.Errorf("INVALID_NAME: theme name is required")), nil, nil
		}
		if !args.Confirm {
			return toolError(fmt.Errorf("VALIDATION_ERROR: confirm:true is required for this destructive decision")), nil, nil
		}
		data, meta, err := c.ThemeDelete(ctx, strings.TrimSpace(args.Name))
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}
