package infrastructure_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/aristorinjuang/lesstruct/modules/themegen/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIThemeServiceGenerate(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantName   string
		wantFiles  map[string]string
		wantErr    bool
	}{
		{
			name:       "success - generates theme",
			response:   `{"choices":[{"message":{"content":"{\"name\":\"midnight-serif\",\"rationale\":\"Dark.\",\"files\":{\"static/base.css\":\":root{}\"}}"}}]}`,
			statusCode: http.StatusOK,
			wantName:   "midnight-serif",
			wantFiles:  map[string]string{"static/base.css": ":root{}"},
			wantErr:    false,
		},
		{
			name:       "success - keeps files verbatim for validation and repair",
			response:   `{"choices":[{"message":{"content":"{\"name\":\"airy-light\",\"rationale\":\"Light.\",\"files\":{\"static/base.css\":\":root{\\n  --color-bg: #fff;\\n}\"}}"}}]}`,
			statusCode: http.StatusOK,
			wantName:   "airy-light",
			wantFiles:  map[string]string{"static/base.css": ":root{\n  --color-bg: #fff;\n}"},
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
			svc := infrastructure.NewOpenAIThemeService("key", server.URL, "model")
			brief := themegendomain.NewBrief("A dark theme", "dark", false)
			theme, err := svc.GenerateTheme(context.Background(), brief, themegendomain.EditionBlog, themegendomain.ThemeSources{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, theme.Name)
			assert.Equal(t, tt.wantFiles, theme.Files)
			var _ = json.RawMessage{}
		})
	}
}

func TestOpenAIThemeServiceRepair(t *testing.T) {
	tests := []struct {
		name       string
		brief      themegendomain.Brief
		files      map[string]string
		problems   []string
		wantPrompt []string
		wantFiles  map[string]string
		wantErr    bool
	}{
		{
			name:       "success - repairs from readable files with the style hint",
			brief:      themegendomain.NewBrief("A dark theme", "dark", false),
			files:      map[string]string{"static/base.css": ":root{\n  --color-bg: #fff;\n}"},
			problems:   []string{"css-parse: static/base.css has unbalanced braces"},
			wantPrompt: []string{"Style preset: dark", "css-parse", "--color-bg"},
			wantFiles:  map[string]string{"static/base.css": ":root{\n  --color-bg: #111;\n}"},
			wantErr:    false,
		},
		{
			name:     "error - invalid envelope",
			brief:    themegendomain.NewBrief("A dark theme", "", false),
			files:    map[string]string{"static/base.css": ":root{}"},
			problems: []string{"css-parse: static/base.css has unbalanced braces"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody []byte
			response := `{"choices":[{"message":{"content":"not json"}}]}`
			if !tt.wantErr {
				envelopeJSON, err := json.Marshal(map[string]any{
					"name":      "midnight-serif",
					"rationale": "Repaired.",
					"files":     tt.wantFiles,
				})
				require.NoError(t, err)
				content, err := json.Marshal(string(envelopeJSON))
				require.NoError(t, err)
				response = `{"choices":[{"message":{"content":` + string(content) + `}}]}`
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				gotBody, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			svc := infrastructure.NewOpenAIThemeService("key", server.URL, "model")
			theme := themegendomain.NewTheme("broken", themegendomain.EditionBlog, tt.files, "")
			repaired, err := svc.RepairTheme(context.Background(), tt.brief, themegendomain.ThemeSources{}, theme, tt.problems)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantFiles, repaired.Files)
			for _, want := range tt.wantPrompt {
				assert.Contains(t, string(gotBody), want)
			}
		})
	}
}
