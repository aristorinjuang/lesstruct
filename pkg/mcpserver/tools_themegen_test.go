package mcpserver_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThemeTools_Generate(t *testing.T) {
	generateBody := `{"data":{"theme":{"name":"midnight-serif","edition":"blog","rationale":"A dark editorial theme.","files":{"static/base.css":":root{--color-bg:#111}","templates/index.html":"body"}},"validation":{"valid":true,"findings":[{"rule":"css-url-external","severity":"warning","message":"external URL"}]}}}`
	gateFailureBody := `{"error":{"code":"generation_failed","message":"Failed to generate theme","details":{"valid":false,"findings":[{"rule":"css-parse","severity":"error","message":"static/style.css has unbalanced braces"}]}}}`

	tests := []struct {
		name        string
		args        map[string]any
		body        string
		status      int
		wantErr     bool
		wantText    []string
		absentText  []string
		wantPayload map[string]any
	}{
		{
			name: "success - returns curated projection without file bodies",
			args: map[string]any{
				"description": "A dark theme", "style": "dark", "templateOverrides": true, "edition": "blog",
			},
			body:    generateBody,
			status:  http.StatusOK,
			wantErr: false,
			wantText: []string{
				`"name":"midnight-serif"`, `"rationale":"A dark editorial theme."`,
				`"fileNames":["static/base.css","templates/index.html"]`,
				`"findings":["warning: external URL"]`,
			},
			absentText: []string{":root{--color-bg:#111}"},
			wantPayload: map[string]any{
				"description": "A dark theme", "style": "dark",
				"templateOverrides": true, "edition": "blog",
			},
		},
		{
			name:    "error - gate failure reports the first finding",
			args:    map[string]any{"description": "A broken theme"},
			body:    gateFailureBody,
			status:  http.StatusInternalServerError,
			wantErr: true,
			wantText: []string{
				"GENERATION_FAILED: static/style.css has unbalanced braces",
			},
		},
		{
			name:    "error - missing description is refused",
			args:    map[string]any{"description": ""},
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
			text, isErr := callTool(t, csess, "theme_generate", tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			for _, want := range tt.wantText {
				assert.Contains(t, text, want)
			}
			for _, absent := range tt.absentText {
				assert.NotContains(t, text, absent)
			}
			if tt.wantPayload != nil {
				assert.Equal(t, http.MethodPost, h.gotMethod)
				assert.Equal(t, "/api/v1/themegen/generate", h.gotPath)
				for key, want := range tt.wantPayload {
					assert.Equal(t, want, h.gotPayload[key])
				}
			}
		})
	}
}

func TestThemeTools_Manage(t *testing.T) {
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
			name:       "success - theme_list returns the envelope",
			tool:       "theme_list",
			args:       map[string]any{},
			body:       `{"data":{"themes":[{"name":"midnight-serif","active":true}]}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"name":"midnight-serif"`},
			wantMethod: http.MethodGet,
			wantPath:   "/api/v1/themegen/themes",
		},
		{
			name:       "success - theme_activate applies",
			tool:       "theme_activate",
			args:       map[string]any{"name": "midnight-serif"},
			body:       `{"data":{"activated":true,"requiresRestart":false}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"requiresRestart":false`},
			wantMethod: http.MethodPost,
			wantPath:   "/api/v1/themegen/themes/midnight-serif/activate",
		},
		{
			name:     "error - theme_activate rejects an empty name",
			tool:     "theme_activate",
			args:     map[string]any{"name": "  "},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"INVALID_NAME"},
		},
		{
			name:       "success - theme_deactivate returns to embedded",
			tool:       "theme_deactivate",
			args:       map[string]any{},
			body:       `{"data":{"activated":false,"requiresRestart":false}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"activated":false`},
			wantMethod: http.MethodPost,
			wantPath:   "/api/v1/themegen/themes/deactivate",
		},
		{
			name:       "success - theme_delete removes with confirm",
			tool:       "theme_delete",
			args:       map[string]any{"name": "paper-light", "confirm": true},
			body:       `{"data":{"deleted":true}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"deleted":true`},
			wantMethod: http.MethodDelete,
			wantPath:   "/api/v1/themegen/themes/paper-light",
		},
		{
			name:     "error - theme_delete refuses without confirm",
			tool:     "theme_delete",
			args:     map[string]any{"name": "paper-light"},
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
			if tt.wantMethod != "" {
				require.Equal(t, tt.wantMethod, h.gotMethod)
				assert.Equal(t, tt.wantPath, h.gotPath)
			}
		})
	}
}
