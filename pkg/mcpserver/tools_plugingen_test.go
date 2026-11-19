package mcpserver_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginToolsGenerate(t *testing.T) {
	generateBody := `{"data":{"project":{"name":"my-plugin","language":"go","rationale":"Uppercases titles.","hooks":["before_save"],"capabilities":["log_info"],"httpPatterns":["https://api.example.com/*"],"dbPermissions":["read:content"],"files":{"main.go":"package main","go.mod":"module x"}},"validation":{"valid":true,"findings":[]}}}`
	gateFailureBody := `{"error":{"code":"generation_failed","message":"Failed to generate plugin","details":{"valid":false,"findings":[{"rule":"go-hook","severity":"error","message":"main.go must export at least one hook"}]}}}`

	tests := []struct {
		name       string
		args       map[string]any
		body       string
		status     int
		wantErr    bool
		wantText   []string
		absentText []string
	}{
		{
			name: "success - returns curated projection without file bodies",
			args: map[string]any{
				"description": "Uppercase titles", "hooks": []any{"before_save"},
			},
			body:    generateBody,
			status:  http.StatusOK,
			wantErr: false,
			wantText: []string{
				`"name":"my-plugin"`, `"fileNames":["go.mod","main.go"]`,
				`"httpPatterns":["https://api.example.com/*"]`, `"dbPermissions":["read:content"]`,
			},
			absentText: []string{"package main"},
		},
		{
			name:    "error - gate failure reports the first finding",
			args:    map[string]any{"description": "A broken plugin", "hooks": []any{"before_save"}},
			body:    gateFailureBody,
			status:  http.StatusInternalServerError,
			wantErr: true,
			wantText: []string{
				"GENERATION_FAILED: main.go must export at least one hook",
			},
		},
		{
			name:    "error - missing description is refused",
			args:    map[string]any{"description": "", "hooks": []any{"before_save"}},
			body:    generateBody,
			status:  http.StatusOK,
			wantErr: true,
			wantText: []string{
				"description is required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, "plugin_generate", tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			for _, want := range tt.wantText {
				assert.Contains(t, text, want)
			}
			for _, absent := range tt.absentText {
				assert.NotContains(t, text, absent)
			}
		})
	}
}

func TestPluginToolsManage(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		args       map[string]any
		body       string
		status     int
		wantErr    bool
		wantText   []string
		wantMethod string
		wantPath   string
	}{
		{
			name:       "success - plugin_projects returns the envelope",
			tool:       "plugin_projects",
			args:       map[string]any{},
			body:       `{"data":{"projects":[{"name":"my-plugin"}]}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"name":"my-plugin"`},
			wantMethod: http.MethodGet,
			wantPath:   "/api/v1/plugingen/projects",
		},
		{
			name:       "success - plugin_status returns loaded plugins",
			tool:       "plugin_status",
			args:       map[string]any{},
			body:       `{"data":{"plugins":[]}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"plugins":[]`},
			wantMethod: http.MethodGet,
			wantPath:   "/api/v1/plugins",
		},
		{
			name:       "success - plugin_build installs",
			tool:       "plugin_build",
			args:       map[string]any{"name": "my-plugin"},
			body:       `{"data":{"installed":true,"requiresRestart":false}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"installed":true`},
			wantMethod: http.MethodPost,
			wantPath:   "/api/v1/plugingen/projects/my-plugin/build",
		},
		{
			name:       "success - plugin_delete removes with confirm",
			tool:       "plugin_delete",
			args:       map[string]any{"name": "my-plugin", "confirm": true},
			body:       `{"data":{"deleted":true}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"deleted":true`},
			wantMethod: http.MethodDelete,
			wantPath:   "/api/v1/plugingen/projects/my-plugin",
		},
		{
			name:     "error - plugin_delete refuses without confirm",
			tool:     "plugin_delete",
			args:     map[string]any{"name": "my-plugin"},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"confirm:true is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			for _, want := range tt.wantText {
				assert.Contains(t, text, want)
			}
		})
	}
}
