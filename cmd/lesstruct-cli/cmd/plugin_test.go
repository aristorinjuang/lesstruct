package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPluginServer(t *testing.T, routes map[string]themeStub, calls *[]themeCall) *httptest.Server {
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

func TestPluginCommands(t *testing.T) {
	generateBody := `{"data":{"project":{"name":"my-plugin","language":"go","rationale":"Uppercases titles.","hooks":["before_save"],"files":{"main.go":"package main"}},"validation":{"valid":true,"findings":[]}},"meta":{}}`
	listBody := `{"data":{"projects":[{"name":"my-plugin","language":"go","hooks":["before_save"],"createdAt":"2026-01-01T00:00:00Z","source":"generated"}]},"meta":{}}`

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
			args: []string{"generate", "--description", "Uppercase titles", "--hooks", "before_save"},
			routes: map[string]themeStub{
				"POST /api/v1/plugingen/generate": {status: http.StatusOK, body: generateBody},
			},
			wantCode: 0,
			wantOut:  []string{"Generated plugin: my-plugin"},
		},
		{
			name: "success - list prints project",
			args: []string{"list"},
			routes: map[string]themeStub{
				"GET /api/v1/plugingen/projects": {status: http.StatusOK, body: listBody},
			},
			wantCode: 0,
			wantOut:  []string{"my-plugin"},
		},
		{
			name: "success - build installs without restart",
			args: []string{"build", "my-plugin"},
			routes: map[string]themeStub{
				"POST /api/v1/plugingen/projects/my-plugin/build": {status: http.StatusOK, body: `{"data":{"installed":true,"requiresRestart":false}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Installed plugin: my-plugin"},
		},
		{
			name: "success - build asks for restart",
			args: []string{"build", "my-plugin"},
			routes: map[string]themeStub{
				"POST /api/v1/plugingen/projects/my-plugin/build": {status: http.StatusOK, body: `{"data":{"installed":true,"requiresRestart":true}}`},
			},
			wantCode: 0,
			wantOut:  []string{"Installed plugin: my-plugin", "Restart the server to apply."},
		},
		{
			name:       "error - build without name",
			args:       []string{"build", "  "},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"plugin name is required"},
		},
		{
			name:       "error - build rejects malformed name",
			args:       []string{"build", "../evil"},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"invalid plugin name"},
		},
		{
			name:       "error - generate without hooks",
			args:       []string{"generate", "--description", "Uppercase titles"},
			routes:     map[string]themeStub{},
			wantCode:   2,
			wantErrOut: []string{"--hooks is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []themeCall
			srv := newPluginServer(t, tt.routes, &calls)
			var out, errOut bytes.Buffer
			code := cmd.ExecuteArgs(themeBaseArgs(srv, append([]string{"plugin"}, tt.args...)...), strings.NewReader(""), &out, &errOut)
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
