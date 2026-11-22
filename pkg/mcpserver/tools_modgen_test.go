package mcpserver_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModuleToolsGenerate(t *testing.T) {
	generateBody := `{"data":{"project":{"name":"forum","surfaces":["routes"],"rationale":"Forum.","files":{"go.mod":"module x","module.go":"package forum","README.md":"r"}},"validation":{"valid":true,"findings":[]}}}`
	gateFailureBody := `{"error":{"code":"generation_failed","message":"Failed to generate module","details":{"valid":false,"findings":[{"rule":"module-go-parse","severity":"error","message":"module.go does not parse"}]}}}`

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
				"description": "A forum module", "surfaces": []any{"routes"},
			},
			body:    generateBody,
			status:  http.StatusOK,
			wantErr: false,
			wantText: []string{
				`"name":"forum"`, `"fileNames":["README.md","go.mod","module.go"]`,
				`"surfaces":["routes"]`,
			},
			absentText: []string{"package forum"},
		},
		{
			name:    "error - gate failure reports the first finding",
			args:    map[string]any{"description": "A broken module"},
			body:    gateFailureBody,
			status:  http.StatusInternalServerError,
			wantErr: true,
			wantText: []string{
				"GENERATION_FAILED: module.go does not parse",
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
			text, isErr := callTool(t, csess, "module_generate", tt.args)
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

func TestModuleToolsManage(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		args        map[string]any
		body        string
		status      int
		wantErr     bool
		wantText    []string
		wantMethod  string
		wantPath    string
		wantPayload map[string]any
	}{
		{
			name:       "success - module_projects returns the envelope",
			tool:       "module_projects",
			args:       map[string]any{},
			body:       `{"data":{"projects":[{"name":"forum"}]}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"name":"forum"`},
			wantMethod: http.MethodGet,
			wantPath:   "/api/v1/modgen/projects",
		},
		{
			name:       "success - module_apply stages the project",
			tool:       "module_apply",
			args:       map[string]any{"name": "forum", "confirm": true},
			body:       `{"data":{"project":{"name":"forum"},"apply":{"applied":true,"verified":true,"steps":[],"appliedModules":["forum"],"requiresRestart":true}}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"applied":true`},
			wantMethod: http.MethodPost,
			wantPath:   "/api/v1/modgen/projects/forum/apply",
			wantPayload: map[string]any{
				"confirm": true,
			},
		},
		{
			name:       "success - module_unapply removes with confirm",
			tool:       "module_unapply",
			args:       map[string]any{"name": "forum", "confirm": true},
			body:       `{"data":{"unapply":{"removed":true,"verified":true,"steps":[],"appliedModules":[],"moduleDirRemoved":true}}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"removed":true`},
			wantMethod: http.MethodPost,
			wantPath:   "/api/v1/modgen/projects/forum/unapply",
		},
		{
			name:     "error - module_unapply refuses without confirm",
			tool:     "module_unapply",
			args:     map[string]any{"name": "forum"},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"confirm:true is required"},
		},
		{
			name:     "error - module_unapply refuses empty name",
			tool:     "module_unapply",
			args:     map[string]any{"name": "", "confirm": true},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"project name is required"},
		},
		{
			name:     "error - module_apply refuses empty name",
			tool:     "module_apply",
			args:     map[string]any{"name": "", "confirm": true},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"project name is required"},
		},
		{
			name:     "error - module_apply refuses without confirm",
			tool:     "module_apply",
			args:     map[string]any{"name": "forum"},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"confirm:true is required"},
		},
		{
			name:       "success - module_delete removes with confirm",
			tool:       "module_delete",
			args:       map[string]any{"name": "forum", "confirm": true},
			body:       `{"data":{"deleted":true}}`,
			status:     http.StatusOK,
			wantErr:    false,
			wantText:   []string{`"deleted":true`},
			wantMethod: http.MethodDelete,
			wantPath:   "/api/v1/modgen/projects/forum",
		},
		{
			name:     "error - module_delete refuses without confirm",
			tool:     "module_delete",
			args:     map[string]any{"name": "forum"},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"confirm:true is required"},
		},
		{
			name:     "error - module_delete refuses empty name",
			tool:     "module_delete",
			args:     map[string]any{"name": "", "confirm": true},
			body:     `{"data":{}}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: []string{"project name is required"},
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
				assert.Equal(t, tt.wantMethod, h.gotMethod)
				assert.Equal(t, tt.wantPath, h.gotPath)
			}
			for key, value := range tt.wantPayload {
				assert.Equal(t, value, h.gotPayload[key], "payload key %s", key)
			}
		})
	}
}
