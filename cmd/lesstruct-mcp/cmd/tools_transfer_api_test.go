package cmd_test

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransferTools_Import(t *testing.T) {
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
			name:     "success - import_wordpress starts job",
			tool:     "import_wordpress",
			args:     map[string]any{"filename": "export.xml", "contentBase64": "PGJvZHk="},
			body:     `{"data":{"jobId":"w1","state":"running"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"jobId":"w1"`,
		},
		{
			name:     "success - import_hugo starts job",
			tool:     "import_hugo",
			args:     map[string]any{"filename": "site.tar.gz", "contentBase64": "PGJvZHk=", "skipMedia": true},
			body:     `{"data":{"jobId":"h1","state":"running"}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"jobId":"h1"`,
		},
		{
			name:     "error - import_wordpress needs filename",
			tool:     "import_wordpress",
			args:     map[string]any{"filename": "", "contentBase64": "PGJvZHk="},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "VALIDATION_ERROR",
		},
		{
			name:     "error - import_hugo rejects bad base64",
			tool:     "import_hugo",
			args:     map[string]any{"filename": "site.tar.gz", "contentBase64": "!!!"},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "VALIDATION_ERROR",
		},
		{
			name:     "success - import_status reports progress",
			tool:     "import_status",
			args:     map[string]any{"jobId": "w1"},
			body:     `{"data":{"state":"done","imported":10,"total":10}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"imported":10`,
		},
		{
			name:     "success - hugo_import_status reports progress",
			tool:     "hugo_import_status",
			args:     map[string]any{},
			body:     `{"data":{"state":"done","imported":3,"total":3}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"imported":3`,
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

func TestTransferTools_ImportUploadShape(t *testing.T) {
	h := &backendHandler{
		body:   `{"data":{"jobId":"w1","state":"running"}}`,
		status: http.StatusOK,
	}
	csess := testSession(t, h)
	text, isErr := callTool(t, csess, "import_wordpress", map[string]any{
		"filename":      "export.xml",
		"contentBase64": base64.StdEncoding.EncodeToString([]byte("<body")),
	})
	assert.False(t, isErr)
	assert.Contains(t, text, `"jobId":"w1"`)
	assert.Equal(t, http.MethodPost, h.gotMethod)
	assert.Equal(t, "/api/v1/wordpress/import", h.gotPath)
	assert.Equal(t, []byte("<body"), h.gotFile)
	assert.Equal(t, "export.xml", h.gotFilename)
}

func TestTransferTools_Archive(t *testing.T) {
	tests := []struct {
		name         string
		tool         string
		wantErr      bool
		wantFilename string
		wantBytes    string
	}{
		{
			name:         "success - export_content returns base64 tarball",
			tool:         "export_content",
			wantErr:      false,
			wantFilename: "export.tar.gz",
			wantBytes:    "tar-bytes",
		},
		{
			name:         "success - ssg_generate returns base64 tarball",
			tool:         "ssg_generate",
			wantErr:      false,
			wantFilename: "site.tar.gz",
			wantBytes:    "site-bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.wantBytes, status: http.StatusOK}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, map[string]any{})
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantFilename)
			// The tarball rides base64 inside the JSON result; it must decode
			// back to the exact bytes the server sent.
			assert.Contains(t, text, base64.StdEncoding.EncodeToString([]byte(tt.wantBytes)))
		})
	}
}

func TestAPITools_EscapeHatch(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		body     string
		status   int
		wantErr  bool
		wantText string
	}{
		{
			name: "success - GET with query passes through",
			args: map[string]any{
				"method": "GET",
				"path":   "/api/v1/orders",
				"query":  map[string]any{"status": "pending"},
			},
			body:     `{"data":[{"id":1}],"meta":{}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
		},
		{
			name: "success - POST with body passes through",
			args: map[string]any{
				"method": "POST",
				"path":   "/api/v1/orders/1/status",
				"body":   map[string]any{"status": "shipped"},
			},
			body:     `{"data":{"order":{"id":1}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
		},
		{
			name: "success - lowercase method normalizes",
			args: map[string]any{
				"method": "get",
				"path":   "/api/v1/content",
			},
			body:     `{"data":[]}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"data":[]`,
		},
		{
			name: "error - unknown method rejected",
			args: map[string]any{
				"method": "FETCH",
				"path":   "/api/v1/content",
			},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "VALIDATION_ERROR",
		},
		{
			name: "error - non-v1 path rejected",
			args: map[string]any{
				"method": "GET",
				"path":   "/admin",
			},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "must start with /api/v1/",
		},
		{
			name: "error - error envelope surfaces",
			args: map[string]any{
				"method": "GET",
				"path":   "/api/v1/orders/9",
			},
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
			text, isErr := callTool(t, csess, "lesstruct_api", tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
		})
	}
}
