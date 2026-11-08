package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// apiArgs carries the `lesstruct_api` escape-hatch payload: any /api/v1
// route the curated tools do not cover yet. Prefer a curated tool when one
// exists — it validates inputs and documents the contract.
type apiArgs struct {
	Method string            `json:"method" jsonschema:"HTTP method: GET, POST, PUT, PATCH, or DELETE"`
	Path   string            `json:"path" jsonschema:"route path, must start with /api/v1/ (e.g. /api/v1/content)"`
	Query  map[string]string `json:"query,omitempty" jsonschema:"query parameters"`
	Body   map[string]any    `json:"body,omitempty" jsonschema:"JSON request body (POST/PUT/PATCH)"`
}

// registerAPITools adds the generic escape-hatch tool to ms. It closes over
// the shared /api/v1 client.
func registerAPITools(ms *mcp.Server, c *client.Client) {
	mcp.AddTool(ms, &mcp.Tool{
		Name:        "lesstruct_api",
		Description: "Escape hatch: call any /api/v1 route the curated tools do not cover. Prefer a curated tool when one exists.",
	}, makeAPIHandler(c))
}

// makeAPIHandler returns the `lesstruct_api` handler.
func makeAPIHandler(
	c *client.Client,
) func(context.Context, *mcp.CallToolRequest, apiArgs) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args apiArgs,
	) (*mcp.CallToolResult, any, error) {
		method, err := validateMCPMethod(args.Method)
		if err != nil {
			return toolError(err), nil, nil
		}
		if !strings.HasPrefix(args.Path, "/api/v1/") {
			return toolError(fmt.Errorf("VALIDATION_ERROR: path must start with /api/v1/, got %q", args.Path)), nil, nil
		}
		query := url.Values{}
		for k, v := range args.Query {
			query.Set(k, v)
		}
		var body any
		if len(args.Body) > 0 {
			body = args.Body
		}
		data, meta, err := c.Do(ctx, method, args.Path, query, body)
		if err != nil {
			return toolError(err), nil, nil
		}
		return toolResult(data, meta), nil, nil
	}
}

// validateMCPMethod normalizes the escape-hatch method and rejects anything
// outside the /api/v1 surface's verbs.
func validateMCPMethod(method string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return strings.ToUpper(strings.TrimSpace(method)), nil
	default:
		return "", fmt.Errorf("VALIDATION_ERROR: invalid method %q (want GET, POST, PUT, PATCH, or DELETE)", method)
	}
}
