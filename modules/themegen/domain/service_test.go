package themegen_test

import (
	"strings"
	"testing"

	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateThemeName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "success - simple slug",
			input:   "midnight-serif",
			wantErr: false,
		},
		{
			name:    "success - minimal length",
			input:   "ab",
			wantErr: false,
		},
		{
			name:    "error - reserved default",
			input:   "default",
			wantErr: true,
		},
		{
			name:    "error - reserved theme-default",
			input:   "theme-default",
			wantErr: true,
		},
		{
			name:    "error - reserved theme-storefront",
			input:   "theme-storefront",
			wantErr: true,
		},
		{
			name:    "error - too short",
			input:   "a",
			wantErr: true,
		},
		{
			name:    "error - uppercase",
			input:   "Midnight",
			wantErr: true,
		},
		{
			name:    "error - leading dash",
			input:   "-midnight",
			wantErr: true,
		},
		{
			name:    "error - empty",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := themegendomain.ValidateThemeName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateThemeFiles(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name: "success - base css only",
			input: map[string]string{
				"static/base.css": ":root{--color-bg:#fff}",
			},
			wantErr: false,
		},
		{
			name: "success - both css",
			input: map[string]string{
				"static/base.css":  ":root{}",
				"static/style.css": ".btn{}",
			},
			wantErr: false,
		},
		{
			name: "error - missing css",
			input: map[string]string{
				"README.md": "hello",
			},
			wantErr: true,
		},
		{
			name:    "error - empty map",
			input:   map[string]string{},
			wantErr: true,
		},
		{
			name: "error - traversal",
			input: map[string]string{
				"static/base.css": ":root{}",
				"../evil.css":     "x",
			},
			wantErr: true,
		},
		{
			name: "error - absolute path",
			input: map[string]string{
				"static/base.css": ":root{}",
				"/etc/passwd":     "x",
			},
			wantErr: true,
		},
		{
			name: "error - dotfile",
			input: map[string]string{
				"static/base.css": ":root{}",
				"static/.hidden":  "x",
			},
			wantErr: true,
		},
		{
			name: "error - per-file cap",
			input: map[string]string{
				"static/base.css":  ":root{}",
				"static/style.css": strings.Repeat("a", (512<<10)+1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := themegendomain.ValidateThemeFiles(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestScanGeneratedThemeFiles(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]string
		wantRules []string
	}{
		{
			name: "success - clean stylesheets",
			input: map[string]string{
				"static/base.css":  ":root{}",
				"static/style.css": ".btn{color:red}",
			},
			wantRules: nil,
		},
		{
			name: "success - clean template",
			input: map[string]string{
				"static/base.css":      ":root{}",
				"templates/index.html": `{{define "body"}}<main><h1>Hi</h1></main>{{end}}`,
			},
			wantRules: nil,
		},
		{
			name: "error - inline script in template",
			input: map[string]string{
				"static/base.css":      ":root{}",
				"templates/index.html": `{{define "body"}}<main><script>alert(1)</script></main>{{end}}`,
			},
			wantRules: []string{"template-script"},
		},
		{
			name: "error - uppercase script tag",
			input: map[string]string{
				"static/base.css":     ":root{}",
				"templates/post.html": `{{define "body"}}<SCRIPT SRC="/static/x.js"></SCRIPT>{{end}}`,
			},
			wantRules: []string{"template-script"},
		},
		{
			name: "error - event handler in template",
			input: map[string]string{
				"static/base.css":      ":root{}",
				"templates/index.html": `{{define "body"}}<img src="/static/a.webp" onerror="alert(1)">{{end}}`,
			},
			wantRules: []string{"template-handler"},
		},
		{
			name: "error - javascript url in template",
			input: map[string]string{
				"static/base.css":      ":root{}",
				"templates/index.html": `{{define "body"}}<a href="JaVaScRiPt:alert(1)">x</a>{{end}}`,
			},
			wantRules: []string{"template-js-url"},
		},
		{
			name: "error - script in static html",
			input: map[string]string{
				"static/base.css":   ":root{}",
				"static/about.html": `<script src="/static/evil.js"></script>`,
			},
			wantRules: []string{"template-script"},
		},
		{
			name: "error - handler in static svg",
			input: map[string]string{
				"static/base.css": ":root{}",
				"static/logo.svg": `<svg onload="alert(1)"></svg>`,
			},
			wantRules: []string{"template-handler"},
		},
		{
			name: "error - root file in generated theme",
			input: map[string]string{
				"static/base.css": ":root{}",
				"root/sw.js":      "self.addEventListener('fetch',()=>{})",
			},
			wantRules: []string{"theme-root-file"},
		},
		{
			name: "error - findings sort by path",
			input: map[string]string{
				"templates/index.html": `{{define "body"}}<script>alert(1)</script>{{end}}`,
				"static/base.css":      ":root{}",
				"root/sw.js":           "x",
			},
			wantRules: []string{"theme-root-file", "template-script"},
		},
		{
			name: "success - data attribute mentioning handler is ignored",
			input: map[string]string{
				"static/base.css":      ":root{}",
				"templates/index.html": `{{define "body"}}<div data-onclick="hint">x</div>{{end}}`,
			},
			wantRules: nil,
		},
		{
			name: "success - stylesheet content is never scanned",
			input: map[string]string{
				"static/base.css":  ":root{}",
				"static/style.css": ".btn{color:red}",
			},
			wantRules: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := themegendomain.ScanGeneratedThemeFiles(tt.input)
			var rules []string
			for _, finding := range findings {
				assert.Equal(t, themegendomain.FindingSeverityError, finding.Severity)
				rules = append(rules, finding.Rule)
			}
			assert.Equal(t, tt.wantRules, rules)
		})
	}
}

func TestParseThemeEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		edition  themegendomain.Edition
		expected string
		wantErr  bool
	}{
		{
			name:     "success - valid envelope",
			input:    `{"name":"midnight-serif","rationale":"Dark editorial.","files":{"static/base.css":":root{}","static/style.css":".btn{}"}}`,
			edition:  themegendomain.EditionBlog,
			expected: "midnight-serif",
			wantErr:  false,
		},
		{
			name:    "error - fences rejected via invalid json",
			input:   "Here is your theme:\n```json\n{\"name\":\"x\"}\n```",
			edition: themegendomain.EditionBlog,
			wantErr: true,
		},
		{
			name:    "error - not json",
			input:   "not json at all",
			edition: themegendomain.EditionBlog,
			wantErr: true,
		},
		{
			name:    "error - missing files",
			input:   `{"name":"midnight-serif","rationale":"x","files":{}}`,
			edition: themegendomain.EditionBlog,
			wantErr: true,
		},
		{
			name:    "error - reserved name",
			input:   `{"name":"default","rationale":"x","files":{"static/base.css":":root{}"}}`,
			edition: themegendomain.EditionBlog,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme, err := themegendomain.ParseThemeEnvelope(tt.input, tt.edition)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, theme.Name)
		})
	}
}

func TestValidateBrief(t *testing.T) {
	tests := []struct {
		name    string
		input   themegendomain.Brief
		wantErr bool
	}{
		{
			name:    "success - valid brief",
			input:   themegendomain.NewBrief("A dark editorial theme for a blog", "dark", false),
			wantErr: false,
		},
		{
			name:    "error - empty description",
			input:   themegendomain.NewBrief("  ", "", false),
			wantErr: true,
		},
		{
			name:    "error - too long",
			input:   themegendomain.NewBrief(strings.Repeat("a", 5001), "", false),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := themegendomain.ValidateBrief(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
