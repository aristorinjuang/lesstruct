package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// themeStub is one stubbed route: the response plus the recorded request.
type themeStub struct {
	status int
	body   string
}

// themeCall records one request the stub server received.
type themeCall struct {
	method  string
	path    string
	payload map[string]any
}

func newThemeServer(t *testing.T, routes map[string]themeStub, calls *[]themeCall) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		route, ok := routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		call := themeCall{method: r.Method, path: r.URL.Path}
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &call.payload)
		}
		*calls = append(*calls, call)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(route.status)
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func themeBaseArgs(srv *httptest.Server, args ...string) []string {
	return append(args, "--base-url", srv.URL, "--api-key", "k")
}

func TestThemeCommands(t *testing.T) {
	generateBody := `{"data":{"theme":{"name":"midnight-serif","edition":"blog","rationale":"A dark editorial theme.","files":{"static/base.css":":root{}"}},"validation":{"valid":true,"findings":[{"rule":"css-url-external","severity":"warning","message":"external URL"}]}}}`
	gateFailureBody := `{"error":{"code":"generation_failed","message":"Failed to generate theme","details":{"valid":false,"findings":[{"rule":"css-parse","severity":"error","message":"static/style.css has unbalanced braces"}]}}}`

	tests := []struct {
		name        string
		args        []string
		routes      map[string]themeStub
		wantCode    int
		wantOut     []string
		wantErrOut  []string
		wantPayload map[string]any
	}{
		{
			name: "success - generate prints name rationale and warnings",
			args: []string{"generate", "--description", "A dark theme", "--style", "dark"},
			routes: map[string]themeStub{
				"POST /api/v1/themegen/generate": {status: http.StatusOK, body: generateBody},
			},
			wantCode: 0,
			wantOut: []string{
				"Generated theme: midnight-serif",
				"A dark editorial theme.",
				"warning: external URL",
			},
			wantPayload: map[string]any{"description": "A dark theme", "style": "dark"},
		},
		{
			name: "success - generate with template overrides and json output",
			args: []string{"generate", "--description", "A dark theme", "--template-overrides", "--output", "json"},
			routes: map[string]themeStub{
				"POST /api/v1/themegen/generate": {status: http.StatusOK, body: generateBody},
			},
			wantCode: 0,
			wantOut:  []string{`"name":"midnight-serif"`, `"validation"`},
			wantPayload: map[string]any{
				"description": "A dark theme", "templateOverrides": true,
			},
		},
		{
			name: "error - gate failure reports the first finding",
			args: []string{"generate", "--description", "A broken theme"},
			routes: map[string]themeStub{
				"POST /api/v1/themegen/generate": {status: http.StatusInternalServerError, body: gateFailureBody},
			},
			wantCode:   client.ExitServer,
			wantErrOut: []string{"Generation failed: static/style.css has unbalanced braces"},
		},
		{
			name:       "error - missing description is usage",
			args:       []string{"generate"},
			routes:     map[string]themeStub{},
			wantCode:   client.ExitUsage,
			wantErrOut: []string{"--description is required"},
		},
		{
			name:       "error - bad timeout is usage",
			args:       []string{"generate", "--description", "x", "--timeout", "soon"},
			routes:     map[string]themeStub{},
			wantCode:   client.ExitUsage,
			wantErrOut: []string{"invalid --timeout"},
		},
		{
			name: "success - list prints installed themes",
			args: []string{"list"},
			routes: map[string]themeStub{
				"GET /api/v1/themegen/themes": {status: http.StatusOK, body: `{"data":{"themes":[{"name":"midnight-serif","edition":"blog","createdAt":"2026-11-14T00:00:00Z","active":true,"source":"generated"}]}}`},
			},
			wantCode: 0,
			wantOut:  []string{"midnight-serif", "blog", "active", "generated"},
		},
		{
			name: "success - activate applies immediately",
			args: []string{"activate", "midnight-serif"},
			routes: map[string]themeStub{
				"POST /api/v1/themegen/themes/midnight-serif/activate": {status: http.StatusOK, body: `{"data":{"activated":true,"requiresRestart":false}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Theme applied."},
		},
		{
			name: "success - activate keeps the restart hint",
			args: []string{"activate", "midnight-serif"},
			routes: map[string]themeStub{
				"POST /api/v1/themegen/themes/midnight-serif/activate": {status: http.StatusOK, body: `{"data":{"activated":true,"requiresRestart":true}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Restart to apply"},
		},
		{
			name: "success - deactivate returns to embedded",
			args: []string{"deactivate"},
			routes: map[string]themeStub{
				"POST /api/v1/themegen/themes/deactivate": {status: http.StatusOK, body: `{"data":{"activated":false,"requiresRestart":false}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Embedded theme applied."},
		},
		{
			name: "success - delete removes the theme",
			args: []string{"delete", "paper-light"},
			routes: map[string]themeStub{
				"DELETE /api/v1/themegen/themes/paper-light": {status: http.StatusOK, body: `{"data":{"deleted":true}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Deleted theme: paper-light"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []themeCall
			srv := newThemeServer(t, tt.routes, &calls)

			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(themeBaseArgs(srv, append([]string{"theme"}, tt.args...)...), strings.NewReader(""), &out, &errOut)
			require.Equal(t, tt.wantCode, code, "stderr: %s", errOut.String())
			for _, want := range tt.wantOut {
				assert.Contains(t, out.String(), want)
			}
			for _, want := range tt.wantErrOut {
				assert.Contains(t, errOut.String(), want)
			}
			if tt.wantPayload != nil {
				require.Len(t, calls, 1)
				for key, want := range tt.wantPayload {
					assert.Equal(t, want, calls[0].payload[key])
				}
			}
		})
	}
}

func TestThemeDownload_WritesZip(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		payload  []byte
		wantFile string
		wantErr  bool
	}{
		{
			name:     "success - writes the theme zip",
			args:     []string{"download", "midnight-serif"},
			payload:  []byte("PK\x03\x04theme-zip-bytes"),
			wantFile: "midnight-serif.zip",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/themegen/themes/midnight-serif/download", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tt.payload)
			}))
			defer srv.Close()

			outDir := t.TempDir()
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(
				themeBaseArgs(srv, "theme", "download", "midnight-serif", "--output-dir", outDir),
				strings.NewReader(""),
				&out,
				&errOut,
			)
			if tt.wantErr {
				require.NotEqual(t, 0, code)
				return
			}
			require.Equal(t, 0, code, "stderr: %s", errOut.String())
			assert.Contains(t, out.String(), "Theme downloaded")

			got, err := os.ReadFile(filepath.Join(outDir, tt.wantFile))
			require.NoError(t, err)
			assert.Equal(t, tt.payload, got)
		})
	}
}
