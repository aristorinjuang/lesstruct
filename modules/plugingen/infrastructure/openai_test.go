package infrastructure_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/aristorinjuang/lesstruct/modules/plugingen/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIPluginServiceGenerate(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success - generates project",
			response:   `{"choices":[{"message":{"content":"{\"name\":\"my-plugin\",\"language\":\"go\",\"hooks\":[\"before_save\"],\"rationale\":\"R.\",\"files\":{\"main.go\":\"package main\\nfunc main() {}\",\"go.mod\":\"module x\"}}"}}]}`,
			statusCode: http.StatusOK,
			wantName:   "my-plugin",
			wantErr:    false,
		},
		{
			name:       "error - invalid envelope",
			response:   `{"choices":[{"message":{"content":"not json"}}]}`,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIPluginService("key", server.URL, "model")
			brief := plugingendomain.NewBrief("A plugin", "go", []string{"before_save"}, nil)
			project, err := svc.GenerateProject(context.Background(), brief, plugingendomain.PluginSources{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, project.Name)
		})
	}
}

func TestOpenAIPluginServiceGenerateRetrial(t *testing.T) {
	envelopeJSON, err := json.Marshal(map[string]any{
		"name":      "my-plugin",
		"language":  "go",
		"hooks":     []string{"before_save"},
		"rationale": "R.",
		"files": map[string]string{
			"main.go": "package main\nfunc main() {}",
			"go.mod":  "module x",
		},
	})
	require.NoError(t, err)
	content, err := json.Marshal(string(envelopeJSON))
	require.NoError(t, err)
	goodResponse := `{"choices":[{"message":{"content":` + string(content) + `}}]}`
	badResponse := `{"choices":[{"message":{"content":"not json"}}]}`

	tests := []struct {
		name      string
		responses []string
		wantName  string
		wantCalls int
		wantErr   bool
	}{
		{
			name:      "success - malformed response re-asked once",
			responses: []string{badResponse, goodResponse},
			wantName:  "my-plugin",
			wantCalls: 2,
			wantErr:   false,
		},
		{
			name:      "error - malformed response twice",
			responses: []string{badResponse, badResponse},
			wantCalls: 2,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := badResponse
				if calls < len(tt.responses) {
					response = tt.responses[calls]
				}
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIPluginService("key", server.URL, "model")
			brief := plugingendomain.NewBrief("A plugin", "go", []string{"before_save"}, nil)
			project, err := svc.GenerateProject(context.Background(), brief, plugingendomain.PluginSources{})
			assert.Equal(t, tt.wantCalls, calls)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, project.Name)
		})
	}
}

func TestOpenAIPluginServiceRepair(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - repairs",
			wantErr: false,
		},
		{
			name:    "error - invalid envelope",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := `{"choices":[{"message":{"content":"not json"}}]}`
			if !tt.wantErr {
				envelopeJSON, err := json.Marshal(map[string]any{
					"name":      "my-plugin",
					"language":  "go",
					"hooks":     []string{"before_save"},
					"rationale": "R.",
					"files": map[string]string{
						"main.go": "package main\nfunc main() {}",
						"go.mod":  "module x",
					},
				})
				require.NoError(t, err)
				content, err := json.Marshal(string(envelopeJSON))
				require.NoError(t, err)
				response = `{"choices":[{"message":{"content":` + string(content) + `}}]}`
			}
			var gotBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				gotBody, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIPluginService("key", server.URL, "model")
			brief := plugingendomain.NewBrief("A plugin", "go", []string{"before_save"}, nil)
			project := plugingendomain.NewProject(
				"broken",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main\nfunc main() {}", "go.mod": "module x"},
				"",
			)
			repaired, err := svc.RepairProject(context.Background(), brief, plugingendomain.PluginSources{}, project, []string{"go-hook: missing"})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "my-plugin", repaired.Name)
			assert.Contains(t, string(gotBody), "go-hook")
		})
	}
}

func TestPrompts(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - prompts build",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := plugingendomain.NewPluginSources("contract", "examples")
			system := infrastructure.BuildSystemPrompt(sources)
			assert.Contains(t, system, "WASM")
			brief := plugingendomain.NewBrief("desc", "go", []string{"before_save"}, []string{"log_info"})
			user := infrastructure.BuildUserPrompt(brief)
			assert.Contains(t, user, "before_save")
			repair := infrastructure.BuildRepairPrompt(brief, map[string]string{"main.go": "x"}, []string{"problem"})
			assert.Contains(t, repair, "problem")
			assert.Equal(t, false, tt.wantErr)
		})
	}
}
