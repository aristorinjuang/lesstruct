package infrastructure_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/aristorinjuang/lesstruct/modules/modgen/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func moduleEnvelopeContent(name string) string {
	envelope, _ := json.Marshal(map[string]any{
		"name":      name,
		"surfaces":  []string{"routes"},
		"rationale": "R.",
		"files": map[string]string{
			"go.mod":    "module x",
			"module.go": "package forum",
			"README.md": "r",
		},
	})
	content, _ := json.Marshal(string(envelope))
	return string(content)
}

func TestOpenAIModuleServiceGenerate(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success - generates module",
			response:   `{"choices":[{"message":{"content":` + moduleEnvelopeContent("forum") + `}}]}`,
			statusCode: http.StatusOK,
			wantName:   "forum",
			wantErr:    false,
		},
		{
			name:       "error - invalid envelope",
			response:   `{"choices":[{"message":{"content":"not json"}}]}`,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "error - service failure",
			response:   `{"error":"boom"}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIModuleService("key", server.URL, "model")
			brief := modgendomain.NewBrief("A forum module", []string{"routes"})
			project, err := svc.GenerateProject(context.Background(), brief, modgendomain.ModuleSources{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, project.Name)
		})
	}
}

func TestOpenAIModuleServiceGenerateRetrial(t *testing.T) {
	tests := []struct {
		name      string
		responses []string
		wantName  string
		wantErr   bool
	}{
		{
			name: "success - malformed then repaired",
			responses: []string{
				`{"choices":[{"message":{"content":"not json"}}]}`,
				`{"choices":[{"message":{"content":` + moduleEnvelopeContent("forum") + `}}]}`,
			},
			wantName: "forum",
			wantErr:  false,
		},
		{
			name: "error - malformed twice",
			responses: []string{
				`{"choices":[{"message":{"content":"not json"}}]}`,
				`{"choices":[{"message":{"content":"still not json"}}]}`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.responses[calls]))
				if calls < len(tt.responses)-1 {
					calls++
				}
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIModuleService("key", server.URL, "model")
			brief := modgendomain.NewBrief("A forum module", nil)
			project, err := svc.GenerateProject(context.Background(), brief, modgendomain.ModuleSources{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, project.Name)
		})
	}
}

func TestOpenAIModuleServiceRepairRetrial(t *testing.T) {
	tests := []struct {
		name      string
		responses []string
		wantName  string
		wantErr   bool
	}{
		{
			name: "success - malformed repair re-asked once",
			responses: []string{
				`{"choices":[{"message":{"content":"not json"}}]}`,
				`{"choices":[{"message":{"content":` + moduleEnvelopeContent("fixed") + `}}]}`,
			},
			wantName: "fixed",
			wantErr:  false,
		},
		{
			name: "error - malformed twice",
			responses: []string{
				`{"choices":[{"message":{"content":"not json"}}]}`,
				`{"choices":[{"message":{"content":"still not json"}}]}`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.responses[calls]))
				if calls < len(tt.responses)-1 {
					calls++
				}
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIModuleService("key", server.URL, "model")
			brief := modgendomain.NewBrief("A forum module", nil)
			project, err := svc.RepairProject(context.Background(), brief, modgendomain.ModuleSources{}, modgendomain.Project{}, []string{"fix it"})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, project.Name)
			assert.Equal(t, 2, calls+1)
		})
	}
}

func TestOpenAIModuleServiceRepair(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantName   string
		wantErr    bool
	}{
		{
			name:       "success - repairs module",
			response:   `{"choices":[{"message":{"content":` + moduleEnvelopeContent("fixed") + `}}]}`,
			statusCode: http.StatusOK,
			wantName:   "fixed",
			wantErr:    false,
		},
		{
			name:       "error - repair malformed",
			response:   `{"choices":[{"message":{"content":"not json"}}]}`,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "error - service failure",
			response:   `{"error":"boom"}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIModuleService("key", server.URL, "model")
			brief := modgendomain.NewBrief("A forum module", nil)
			project, err := svc.RepairProject(context.Background(), brief, modgendomain.ModuleSources{}, modgendomain.Project{}, []string{"fix it"})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, project.Name)
		})
	}
}
