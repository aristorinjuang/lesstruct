package mcpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/pkg/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backendHandler is the MCP test backend: a fixed body+status that records
// the request it serves. Multiple registrations are wrapped in a mux by each
// test; multipart uploads are parsed so the transfer tests can assert on the
// file + metadata parts.
type backendHandler struct {
	body        string
	status      int
	gotMethod   string
	gotPath     string
	gotQuery    string
	gotAuth     string
	gotCT       string
	gotPayload  map[string]any
	gotFile     []byte
	gotFilename string
	gotMetadata string
	bodies      map[string]struct {
		body   string
		status int
	}
}

func (h *backendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.gotMethod = r.Method
	h.gotPath = r.URL.Path
	h.gotQuery = r.URL.RawQuery
	h.gotAuth = r.Header.Get("Authorization")
	h.gotCT = r.Header.Get("Content-Type")

	body, status := h.body, h.status
	if h.bodies != nil {
		if route, ok := h.bodies[r.Method+" "+r.URL.Path]; ok {
			body, status = route.body, route.status
		}
	}

	if strings.HasPrefix(h.gotCT, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err == nil {
			if f, fh, ferr := r.FormFile("file"); ferr == nil {
				h.gotFile, _ = io.ReadAll(f)
				h.gotFilename = fh.Filename
			}
			h.gotMetadata = r.FormValue("metadata")
		}
	} else if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &h.gotPayload)
	}

	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// testSession wires a New MCP server to a backend handler and returns a
// connected MCP client session speaking to it over in-memory transports.
func testSession(t *testing.T, h *backendHandler) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	ms, err := mcpserver.New("k", srv.URL, "test")
	require.NoError(t, err)

	serverTrans, clientTrans := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ssess, err := ms.Connect(ctx, serverTrans)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ssess.Close() })

	mc := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	csess, err := mc.Connect(ctx, clientTrans, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = csess.Close() })
	return csess
}

// callTool invokes one tool and returns the first text payload plus the
// IsError flag. Every tool here answers with a single TextContent.
func callTool(
	t *testing.T,
	csess *mcp.ClientSession,
	name string,
	args map[string]any,
) (string, bool) {
	t.Helper()
	res, err := csess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.NotEmpty(t, res.Content)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "tool %s must answer with TextContent", name)
	return text.Text, res.IsError
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		wantErr bool
	}{
		{
			name:    "success - valid base URL",
			apiKey:  "k",
			baseURL: "http://localhost:8080",
			wantErr: false,
		},
		{
			name:    "error - base URL with path",
			apiKey:  "k",
			baseURL: "http://localhost:8080/prefix",
			wantErr: true,
		},
		{
			name:    "error - malformed base URL",
			apiKey:  "k",
			baseURL: "://bad",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := mcpserver.New(tt.apiKey, tt.baseURL, "test")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, srv)
		})
	}
}

func TestContentTools_ListGet(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		body     string
		status   int
		wantErr  bool
		wantText string
	}{
		{
			name:     "success - content_list passes filters",
			tool:     "content_list",
			args:     map[string]any{"limit": 2, "status": "draft"},
			body:     `{"data":[{"id":1}],"meta":{"pagination":{"hasMore":false}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
		},
		{
			name:     "success - content_get returns item",
			tool:     "content_get",
			args:     map[string]any{"id": 7},
			body:     `{"data":{"content":{"id":7,"title":"Hi"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"title":"Hi"`,
		},
		{
			name:     "error - content_get invalid id",
			tool:     "content_get",
			args:     map[string]any{"id": 0},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "INVALID_ID",
		},
		{
			name:     "error - content_get surfaces 404",
			tool:     "content_get",
			args:     map[string]any{"id": 9},
			body:     `{"error":{"code":"NOT_FOUND","message":"nope"}}`,
			status:   http.StatusNotFound,
			wantErr:  true,
			wantText: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
		})
	}
}

func TestContentTools_Create(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		body        string
		status      int
		wantErr     bool
		wantText    string
		wantPayload map[string]any
	}{
		{
			name: "success - markdown create defaults format and derives title",
			args: map[string]any{
				"body":     "# Hello\nWorld",
				"postType": "post",
			},
			body:     `{"data":{"content":{"id":1,"title":"Hello","status":"draft"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
			wantPayload: map[string]any{
				"title":    "Hello",
				"body":     "# Hello\nWorld",
				"format":   "markdown",
				"postType": "post",
			},
		},
		{
			name: "success - explicit title and html format",
			args: map[string]any{
				"body":   "<p>Hi</p>",
				"title":  "T",
				"format": "html",
			},
			body:     `{"data":{"content":{"id":2}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":2`,
			wantPayload: map[string]any{
				"title":  "T",
				"body":   "<p>Hi</p>",
				"format": "html",
			},
		},
		{
			name:        "error - empty body",
			args:        map[string]any{"body": "  "},
			body:        `{}`,
			status:      http.StatusOK,
			wantErr:     true,
			wantText:    "VALIDATION_ERROR",
			wantPayload: nil,
		},
		{
			name:        "error - bad format",
			args:        map[string]any{"body": "x", "format": "docx"},
			body:        `{}`,
			status:      http.StatusOK,
			wantErr:     true,
			wantText:    "VALIDATION_ERROR",
			wantPayload: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, "content_create", tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
			if tt.wantPayload != nil {
				for k, v := range tt.wantPayload {
					assert.Equal(t, v, h.gotPayload[k], "payload key %s", k)
				}
				assert.Equal(t, http.MethodPost, h.gotMethod)
				assert.Equal(t, "/api/v1/content", h.gotPath)
			}
		})
	}
}

func TestContentTools_UpdatePatch(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		wantText    string
		wantPayload map[string]any
		wantAbsent  []string
	}{
		{
			name: "success - body-only edit preserves stored fields",
			args: map[string]any{
				"id":   3,
				"body": "# New",
			},
			wantErr:  false,
			wantText: `"id":3`,
			wantPayload: map[string]any{
				"title":    "New",
				"body":     "# New",
				"format":   "markdown",
				"postType": "post",
				"language": "en",
			},
			wantAbsent: nil,
		},
		{
			name: "success - explicit fields replace",
			args: map[string]any{
				"id":        3,
				"body":      "# New",
				"title":     "Kept",
				"published": true,
			},
			wantErr:  false,
			wantText: `"id":3`,
			wantPayload: map[string]any{
				"title":       "Kept",
				"isPublished": true,
			},
			wantAbsent: nil,
		},
		{
			name: "success - explicit empty tags clear them",
			args: map[string]any{
				"id":   3,
				"body": "# New",
				"tags": []any{},
			},
			wantErr:     false,
			wantText:    `"id":3`,
			wantPayload: nil,
			// An explicit [] normalizes to nil and rides omitted (the
			// shared client uses omitempty, like the CLI); the server
			// normalizes a missing tags key to empty, which clears them.
			wantAbsent: []string{"tags"},
		},
		{
			name:        "error - format without body",
			args:        map[string]any{"id": 3, "format": "html"},
			wantErr:     true,
			wantText:    "VALIDATION_ERROR",
			wantPayload: nil,
			wantAbsent:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{
				bodies: map[string]struct {
					body   string
					status int
				}{
					"GET /api/v1/content/3": {
						body:   `{"data":{"content":{"id":3,"title":"Old","body":"old","format":"tiptap","postType":"post","tags":["a"],"language":"en","status":"draft"}}}`,
						status: http.StatusOK,
					},
					"PUT /api/v1/content/3": {
						body:   `{"data":{"content":{"id":3}}}`,
						status: http.StatusOK,
					},
				},
			}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, "content_update", tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
			if tt.wantPayload != nil {
				for k, v := range tt.wantPayload {
					assert.Equal(t, v, h.gotPayload[k], "payload key %s", k)
				}
			}
			for _, k := range tt.wantAbsent {
				_, ok := h.gotPayload[k]
				assert.False(t, ok, "payload key %s must be absent", k)
			}
		})
	}
}

func TestContentTools_DeletePublishSystemFields(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		body     string
		status   int
		wantErr  bool
		wantText string
	}{
		{
			name:     "success - content_delete confirms",
			tool:     "content_delete",
			args:     map[string]any{"id": 4},
			body:     ``,
			status:   http.StatusNoContent,
			wantErr:  false,
			wantText: `"deleted":true`,
		},
		{
			name:     "error - content_delete invalid id",
			tool:     "content_delete",
			args:     map[string]any{"id": -1},
			body:     ``,
			status:   http.StatusNoContent,
			wantErr:  true,
			wantText: "INVALID_ID",
		},
		{
			name:     "success - content_publish returns item",
			tool:     "content_publish",
			args:     map[string]any{"id": 5},
			body:     `{"data":{"content":{"id":5,"status":"published"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"published"`,
		},
		{
			name:     "success - content_unpublish returns item",
			tool:     "content_unpublish",
			args:     map[string]any{"id": 5},
			body:     `{"data":{"content":{"id":5,"status":"draft"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"draft"`,
		},
		{
			name:     "success - content_system_fields sets fields",
			tool:     "content_system_fields",
			args:     map[string]any{"id": 6, "fields": map[string]any{"editorial_status": "review"}},
			body:     `{"data":{"content":{"id":6}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":6`,
		},
		{
			name:     "error - content_system_fields needs fields",
			tool:     "content_system_fields",
			args:     map[string]any{"id": 6},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
		})
	}
}

func TestMediaTools(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		body     string
		status   int
		wantErr  bool
		wantText string
	}{
		{
			name:     "success - media_list paginates",
			tool:     "media_list",
			args:     map[string]any{"limit": 5},
			body:     `{"data":[{"id":1}],"meta":{"pagination":{"hasMore":false}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
		},
		{
			name:     "success - media_get returns item",
			tool:     "media_get",
			args:     map[string]any{"id": 8},
			body:     `{"data":{"media":{"id":8}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":8`,
		},
		{
			name:     "success - media_delete confirms",
			tool:     "media_delete",
			args:     map[string]any{"id": 8},
			body:     ``,
			status:   http.StatusNoContent,
			wantErr:  false,
			wantText: `"deleted":true`,
		},
		{
			name:     "error - media_delete invalid id",
			tool:     "media_delete",
			args:     map[string]any{"id": 0},
			body:     ``,
			status:   http.StatusNoContent,
			wantErr:  true,
			wantText: "INVALID_ID",
		},
		{
			name:     "success - media_reencode reports",
			tool:     "media_reencode",
			args:     map[string]any{},
			body:     `{"data":{"processed":2,"skipped":1}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"processed":2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
		})
	}
}

func TestMediaTools_Upload(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		wantText    string
		wantFile    string
		wantAltText string
	}{
		{
			name: "success - uploads bytes with filename and alt text",
			args: map[string]any{
				"filename":      "photo.png",
				"contentBase64": "aGVsbG8=",
				"altText":       "a view",
			},
			wantErr:     false,
			wantText:    `"id":9`,
			wantFile:    "hello",
			wantAltText: `"altText":"a view"`,
		},
		{
			name: "error - missing filename",
			args: map[string]any{
				"filename":      "",
				"contentBase64": "aGVsbG8=",
			},
			wantErr:     true,
			wantText:    "VALIDATION_ERROR",
			wantFile:    "",
			wantAltText: "",
		},
		{
			name: "error - bad base64",
			args: map[string]any{
				"filename":      "x.png",
				"contentBase64": "!!!",
			},
			wantErr:     true,
			wantText:    "VALIDATION_ERROR",
			wantFile:    "",
			wantAltText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{
				body:   `{"data":{"media":{"id":9}}}`,
				status: http.StatusOK,
			}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, "media_upload", tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
			assert.Equal(t, tt.wantFile, string(h.gotFile))
			if tt.wantAltText != "" {
				assert.Contains(t, h.gotMetadata, tt.wantAltText)
			}
		})
	}
}
