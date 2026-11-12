package mcp_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcphandler "github.com/aristorinjuang/lesstruct/internal/api/handlers/mcp"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backend is the in-process /api/v1 router stand-in: it records the request
// and answers with a fixed envelope, so the tests can prove the MCP layer ran
// through the same auth/path contract as a real tool call.
type backend struct {
	gotAuth   string
	gotPath   string
	gotMethod string
	calls     int
}

func (b *backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.gotAuth = r.Header.Get("Authorization")
	b.gotPath = r.URL.Path
	b.gotMethod = r.Method
	b.calls++
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"data":{"content":{"id":1,"title":"Hello"}},"error":null}`)
}

// authTransport stamps the API key on every request, mirroring a remote host's
// Bearer configuration (the SDK client has no dedicated header option).
type authTransport struct {
	token string
}

func (a authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	cloned := r.Clone(r.Context())
	cloned.Header.Set("Authorization", "Bearer "+a.token)
	return http.DefaultTransport.RoundTrip(cloned)
}

// connect opens an MCP client session against a handler serving backend. The
// standalone SSE stream is disabled because the endpoint is stateless and
// POST-only.
func connect(t *testing.T, backend http.Handler, token string) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(mcphandler.New(backend))
	t.Cleanup(srv.Close)
	return connectURL(t, srv.URL, token)
}

// connectURL opens an MCP client session against an already-mounted endpoint,
// so a test can wrap the MCP handler in a router of its own.
func connectURL(t *testing.T, endpoint, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &http.Client{Transport: authTransport{token: token}},
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestHandler_InitializeAndListTools(t *testing.T) {
	session := connect(t, &backend{}, "test-key")

	init := session.InitializeResult()
	require.NotNil(t, init)
	assert.Equal(t, "lesstruct-mcp", init.ServerInfo.Name)

	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, tools.Tools, 44)
}

func TestHandler_ToolCallUsesRequestKey(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		tool       string
		args       map[string]any
		wantPath   string
		wantMethod string
	}{
		{
			name:       "success - content_list traverses the in-process router",
			token:      "lesstruct_key_123",
			tool:       "content_list",
			args:       map[string]any{"limit": 5},
			wantPath:   "/api/v1/content",
			wantMethod: http.MethodGet,
		},
		{
			name:       "success - escape hatch forwards a v1 path",
			token:      "lesstruct_key_456",
			tool:       "lesstruct_api",
			args:       map[string]any{"method": http.MethodGet, "path": "/api/v1/content/7"},
			wantPath:   "/api/v1/content/7",
			wantMethod: http.MethodGet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &backend{}
			session := connect(t, b, tt.token)

			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      tt.tool,
				Arguments: tt.args,
			})
			require.NoError(t, err)
			require.False(t, res.IsError)
			require.NotEmpty(t, res.Content)
			text, ok := res.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Contains(t, text.Text, `"id":1`)
			assert.Equal(t, "Bearer "+tt.token, b.gotAuth)
			assert.Equal(t, tt.wantPath, b.gotPath)
			assert.Equal(t, tt.wantMethod, b.gotMethod)
			assert.Equal(t, 1, b.calls)
		})
	}
}

func TestHandler_DistinctKeysStayIsolated(t *testing.T) {
	b := &backend{}
	sessionA := connect(t, b, "key-a")
	sessionB := connect(t, b, "key-b")

	_, err := sessionA.CallTool(context.Background(), &mcp.CallToolParams{Name: "content_list"})
	require.NoError(t, err)
	assert.Equal(t, "Bearer key-a", b.gotAuth)

	_, err = sessionB.CallTool(context.Background(), &mcp.CallToolParams{Name: "content_list"})
	require.NoError(t, err)
	assert.Equal(t, "Bearer key-b", b.gotAuth)
}

func TestHandler_MissingKeyRejected(t *testing.T) {
	srv := httptest.NewServer(mcphandler.New(&backend{}))
	t.Cleanup(srv.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(payload), "no server available")
}

func TestHandler_JSONResponse(t *testing.T) {
	backend := &backend{}
	srv := httptest.NewServer(mcphandler.New(backend))
	t.Cleanup(srv.Close)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer test-key")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var envelope struct {
		Result struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&envelope))
	assert.Equal(t, "lesstruct-mcp", envelope.Result.ServerInfo.Name)
}

func TestHandler_RejectsNonV1ToolPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantText string
	}{
		{
			name:     "error - escape hatch rejects a non-v1 path",
			path:     "/admin/users",
			wantText: "must start with /api/v1/",
		},
		{
			name:     "error - in-process transport rejects a path that escapes v1",
			path:     "/api/v1/../admin/users",
			wantText: "only serves /api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := connect(t, &backend{}, "test-key")

			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "lesstruct_api",
				Arguments: map[string]any{
					"method": http.MethodGet,
					"path":   tt.path,
				},
			})
			require.NoError(t, err)
			require.True(t, res.IsError)
			text, ok := res.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Contains(t, text.Text, tt.wantText)
		})
	}
}

// TestHandler_InnerRequestUsesInnerMethod proves the in-process transport
// drops chi's inherited route context. The tool call runs inside the outer
// POST /mcp request, so reusing that context would make chi dispatch a GET
// /api/v1/content as the POST handler (the crash that motivated this test).
func TestHandler_InnerRequestUsesInnerMethod(t *testing.T) {
	const envelope = `{"data":{"content":{"id":1,"title":"Hello"}},"error":null}`

	var gotMethod string
	api := chi.NewRouter()
	api.Get("/api/v1/content", func(w http.ResponseWriter, _ *http.Request) {
		gotMethod = http.MethodGet
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, envelope)
	})
	api.Post("/api/v1/content", func(w http.ResponseWriter, _ *http.Request) {
		gotMethod = http.MethodPost
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, envelope)
	})

	// The MCP handler sits on a chi route in production too, so the outer
	// request carries RouteMethod=POST into every tool call.
	outer := chi.NewRouter()
	outer.Post("/mcp", mcphandler.New(api).ServeHTTP)

	srv := httptest.NewServer(outer)
	t.Cleanup(srv.Close)
	session := connectURL(t, srv.URL+"/mcp", "test-key")
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "content_list",
		Arguments: map[string]any{"limit": 2},
	})
	require.NoError(t, err)
	if res.IsError {
		text, _ := res.Content[0].(*mcp.TextContent)
		t.Fatalf("tool error: %s", text.Text)
	}
	assert.Equal(t, http.MethodGet, gotMethod)
}
