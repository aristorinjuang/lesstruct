package cmd

import (
	"context"
	"encoding/json"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// serverName is the MCP implementation name agents see in handshake.
	serverName = "lesstruct-mcp"

	// serverInstructions tells agents how to work with this server: Markdown
	// is the authoring format, cursors paginate, and state-changing tools
	// need no confirmation beyond the agent's own judgment.
	serverInstructions = "Lesstruct CMS over MCP. Author content as Markdown " +
		"(the server converts to rich text). List tools paginate with limit/cursor " +
		"(follow meta.pagination.nextCursor while meta.pagination.hasMore is true). " +
		"IDs are numeric. State-changing tools (create/update/delete/transition) " +
		"take effect immediately — confirm with the user first when the action " +
		"is destructive (delete, reject, suspend, soft-delete)."
)

// toolResult renders a successful /api/v1 call as the MCP text result: the
// {data, meta} envelope as JSON. Agents parse cursors and projections from it.
func toolResult(data, meta json.RawMessage) *mcp.CallToolResult {
	env := struct {
		Data json.RawMessage `json:"data,omitempty"`
		Meta json.RawMessage `json:"meta,omitempty"`
	}{Data: data, Meta: meta}
	encoded, err := json.Marshal(env)
	if err != nil {
		return toolError(err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
	}
}

// toolJSON renders an ad-hoc success payload (delete confirmations, base64
// downloads) as the MCP text result.
func toolJSON(v any) *mcp.CallToolResult {
	encoded, err := json.Marshal(v)
	if err != nil {
		return toolError(err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
	}
}

// toolError renders a failed /api/v1 call as an MCP tool error (not a
// transport error): the agent sees the CODE: message line and can react to it
// (retry, ask the user, pick another tool).
func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

// registerTools adds every tool group to ms. All handlers close over the
// shared /api/v1 client, so one API key and base URL serve the whole surface.
func registerTools(ms *mcp.Server, c *client.Client) {
	registerContentTools(ms, c)
	registerMediaTools(ms, c)
	registerCommerceTools(ms, c)
	registerUserTools(ms, c)
	registerCommentTools(ms, c)
	registerTransferTools(ms, c)
	registerAPITools(ms, c)
}

// Server is the lesstruct-mcp MCP server: a stdio JSON-RPC front over the
// shared /api/v1 client. Build it with New, serve it with Run.
type Server struct {
	mcpServer *mcp.Server
	client    *client.Client
}

// Run serves the tool surface over stdio until the client disconnects.
// Stdout is protocol-only; the SDK logs transport noise to stderr.
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// Connect attaches the tool surface to one client session over t and returns
// the session. Run is the stdio specialization; Connect is the seam the tests
// drive with in-memory transports (and any future HTTP transport would share).
func (s *Server) Connect(ctx context.Context, t mcp.Transport) (*mcp.ServerSession, error) {
	return s.mcpServer.Connect(ctx, t, nil)
}

// New builds a Server targeting baseURL with apiKey. The key is resolved by
// the caller (flag → env → config file); the client validates the base URL
// shape. Every tool group registers here, so a new group is one register call
// plus its tools_*.go file.
func New(apiKey, baseURL string) (*Server, error) {
	c, err := client.New(baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	ms := mcp.NewServer(
		&mcp.Implementation{Name: serverName, Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	registerTools(ms, c)
	return &Server{mcpServer: ms, client: c}, nil
}
