package cmd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleCommands(t *testing.T) {
	generateBody := `{"data":{"project":{"name":"forum","surfaces":["routes"],"rationale":"Forum.","files":{"module.go":"package forum"}},"validation":{"valid":true,"findings":[]}},"meta":{}}`
	listBody := `{"data":{"projects":[{"name":"forum","surfaces":["routes"],"fileCount":9,"createdAt":"2026-01-01T00:00:00Z","source":"generated"}]},"meta":{}}`

	tests := []struct {
		name       string
		args       []string
		routes     map[string]themeStub
		wantCode   int
		wantOut    []string
		wantErrOut []string
	}{
		{
			name: "success - generate prints name",
			args: []string{"generate", "--description", "A forum", "--surfaces", "routes"},
			routes: map[string]themeStub{
				"POST /api/v1/modgen/generate": {status: http.StatusOK, body: generateBody},
			},
			wantCode: 0,
			wantOut:  []string{"Generated module: forum"},
		},
		{
			name: "success - list prints project",
			args: []string{"list"},
			routes: map[string]themeStub{
				"GET /api/v1/modgen/projects": {status: http.StatusOK, body: listBody},
			},
			wantCode: 0,
			wantOut:  []string{"forum"},
		},
		{
			name: "success - delete removes project",
			args: []string{"delete", "forum"},
			routes: map[string]themeStub{
				"DELETE /api/v1/modgen/projects/forum": {status: http.StatusOK, body: `{"data":{"deleted":true}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Deleted module: forum"},
		},
		{
			name: "success - apply stages and verifies",
			args: []string{"apply", "forum"},
			routes: map[string]themeStub{
				"POST /api/v1/modgen/projects/forum/apply": {status: http.StatusOK, body: `{"data":{"project":{"name":"forum"},"apply":{"applied":true,"verified":true,"steps":[{"name":"tests","passed":true,"skipped":false},{"name":"build","passed":true,"skipped":false}],"appliedModules":["forum"],"requiresRestart":true}}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Applied module: forum", "tests: pass", "Rebuild"},
		},
		{
			name:       "error - apply replace without confirm",
			args:       []string{"apply", "forum", "--replace"},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"--replace requires --confirm"},
		},
		{
			name: "success - unapply removes with confirm",
			args: []string{"unapply", "forum", "--confirm"},
			routes: map[string]themeStub{
				"POST /api/v1/modgen/projects/forum/unapply": {status: http.StatusOK, body: `{"data":{"unapply":{"removed":true,"verified":true,"steps":[{"name":"build","passed":true,"skipped":false}],"appliedModules":[],"moduleDirRemoved":true}}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Removed module: forum", "build: pass"},
		},
		{
			name:       "error - unapply without confirm",
			args:       []string{"unapply", "forum"},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"--confirm is required"},
		},
		{
			name: "success - apply reports verification failure",
			args: []string{"apply", "forum"},
			routes: map[string]themeStub{
				"POST /api/v1/modgen/projects/forum/apply": {status: http.StatusOK, body: `{"data":{"project":{"name":"forum"},"apply":{"applied":true,"verified":false,"steps":[{"name":"tests","passed":false,"skipped":false,"output":"FAIL"}],"appliedModules":["forum"],"requiresRestart":true}}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Applied module: forum", "Verification failed", "tests: FAIL"},
		},
		{
			name:       "error - generate without description",
			args:       []string{"generate"},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"--description is required"},
		},
		{
			name:       "error - delete without name",
			args:       []string{"delete", "  "},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"module name is required"},
		},
		{
			name:       "error - delete rejects malformed name",
			args:       []string{"delete", "../evil"},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"invalid module name"},
		},
		{
			name:       "error - delete rejects hyphenated name",
			args:       []string{"delete", "my-mod"},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"invalid module name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []themeCall
			srv := newPluginServer(t, tt.routes, &calls)
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(themeBaseArgs(srv, append([]string{"module"}, tt.args...)...), strings.NewReader(""), &out, &errOut)
			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut.String())
			for _, want := range tt.wantOut {
				assert.Contains(t, out.String(), want)
			}
			for _, want := range tt.wantErrOut {
				assert.Contains(t, errOut.String(), want)
			}
		})
	}
}

func TestModuleDownload_WritesZip(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		payload  []byte
		wantFile string
		wantErr  bool
	}{
		{
			name:     "success - writes the module zip",
			args:     []string{"download", "forum"},
			payload:  []byte("PK\x03\x04module-zip-bytes"),
			wantFile: "forum.zip",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/modgen/projects/forum/download", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tt.payload)
			}))
			defer srv.Close()

			outDir := t.TempDir()
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(
				themeBaseArgs(srv, "module", "download", "forum", "--output-dir", outDir),
				strings.NewReader(""),
				&out,
				&errOut,
			)
			if tt.wantErr {
				require.NotEqual(t, 0, code)
				return
			}
			require.Equal(t, 0, code, "stderr: %s", errOut.String())
			assert.Contains(t, out.String(), "Module downloaded")

			got, err := os.ReadFile(filepath.Join(outDir, tt.wantFile))
			require.NoError(t, err)
			assert.Equal(t, tt.payload, got)
		})
	}
}
