package blog

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/stretchr/testify/assert"
)

// hasFindingRule reports whether the report carries a finding with rule.
func hasFindingRule(report themegendomain.ValidationReport, rule string) bool {
	for _, finding := range report.Findings {
		if finding.Rule == rule {
			return true
		}
	}
	return false
}

func TestRenderingValidatorValidate(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		wantValid  bool
		wantRule   string
		wantAbsent string
	}{
		{
			name:       "success - stylesheet only theme skips template gate",
			files:      map[string]string{"static/base.css": ":root{--color-primary:#123456}"},
			wantValid:  true,
			wantAbsent: "template-parse",
		},
		{
			name: "success - valid template override",
			files: map[string]string{
				"static/base.css":      ":root{--color-primary:#123456}",
				"static/style.css":     ".container{max-width:60rem}",
				"templates/index.html": `{{define "body"}}<main class="custom-index"><h1>{{.Title}}</h1></main>{{end}}`,
			},
			wantValid:  true,
			wantAbsent: "template-parse",
		},
		{
			name: "error - unclosed template fails the parse gate",
			files: map[string]string{
				"static/base.css":      ":root{--color-primary:#123456}",
				"templates/index.html": `{{define "body"}}<div>unclosed`,
			},
			wantValid: false,
			wantRule:  "template-parse",
		},
		{
			name: "error - unknown field fails the render gate",
			files: map[string]string{
				"static/base.css":      ":root{--color-primary:#123456}",
				"templates/index.html": `{{define "body"}}<p>{{.MissingField}}</p>{{end}}`,
			},
			wantValid: false,
			wantRule:  "template-render",
		},
		{
			name: "error - stylesheet failure short-circuits template gate",
			files: map[string]string{
				"static/base.css":      ":root{",
				"templates/index.html": `{{define "body"}}<div>unclosed`,
			},
			wantValid:  false,
			wantRule:   "css-parse",
			wantAbsent: "template-parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installTestThemeRegistry(t)
			validator := newThemeValidator([]string{"post"})
			theme := themegendomain.NewTheme(
				"test-theme",
				themegendomain.EditionBlog,
				tt.files,
				"",
			)
			report := validator.Validate(context.Background(), theme)
			assert.Equal(t, tt.wantValid, report.Valid)
			if tt.wantRule != "" {
				assert.True(t, hasFindingRule(report, tt.wantRule))
			}
			if tt.wantAbsent != "" {
				assert.False(t, hasFindingRule(report, tt.wantAbsent))
			}
		})
	}
}

func TestReadThemeTemplateSources(t *testing.T) {
	tests := []struct {
		name         string
		files        fs.FS
		wantContains []string
		wantAbsent   []string
		wantEmpty    bool
	}{
		{
			name:         "success - reads the embedded page templates",
			files:        themedefault.Files(),
			wantContains: []string{"--- template: index ---", "--- template: layout ---", `{{- define "body"}}`},
			wantAbsent:   []string{"static/base.css"},
			wantEmpty:    false,
		},
		{
			name:         "success - nil filesystem yields nothing",
			files:        nil,
			wantContains: []string{},
			wantAbsent:   []string{},
			wantEmpty:    true,
		},
		{
			name:         "success - missing pages dir yields nothing",
			files:        fstest.MapFS{"readme.txt": &fstest.MapFile{Data: []byte("x")}},
			wantContains: []string{},
			wantAbsent:   []string{},
			wantEmpty:    true,
		},
		{
			name: "success - skips non-template files",
			files: fstest.MapFS{
				"pages/index.gohtml": &fstest.MapFile{Data: []byte(`{{define "body"}}{{end}}`)},
				"pages/data.json":    &fstest.MapFile{Data: []byte(`{}`)},
			},
			wantContains: []string{"--- template: index ---", `{{define "body"}}`},
			wantAbsent:   []string{"data.json"},
			wantEmpty:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readThemeTemplateSources(tt.files)
			if tt.wantEmpty {
				assert.Empty(t, got)
				return
			}
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, got, absent)
			}
		})
	}
}
